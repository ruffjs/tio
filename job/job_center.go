package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

// Center Scheduler manage the jobs and tasks according to configs, change the status of job
// Send task to Runner and Runner handle the task Queue, do the action for task
// When task status change, Runner notify it to Center to handle the status and schedule of job
//                             ┌──────────────────────────┐
//                             │         job center       │
//                             │                          │
//                             │   ┌───────────────────┐  │
//     job/task manage ───────►│   │      schedule     │  │
//                             │   │                   │  │
//    load job from db ───────►│   │       timing      │  ├──────► database(save state)
//                             │   │   retry, timeout  │  │
//                             │   └───────────────────┘  │
//                             │           ▼  ▲           │
//                             │   ┌───────────────────┐  │
//                             │   │    state trans    │  │
//                             │   └───────────────────┘  │
//                             │           ▼  ▲           │
//                             │   ┌───────────────────┐  │
//                             │   │     task queue    │  │
//  Device(get/report) ───────►│   └───────────────────┘  ├──────► Action(notify-thing/direct-method/update-shadow)
//                             │                          │
//                             └──────────────────────────┘

const centerWorkerPoolSize = 100
const runnerWorkerPoolSize = 500

// Center Entrance to task operation management
//   - Receive management messages from the upper layer
//   - Schedule job and task (according to schedule config , timeout config ...)
//   - Manage job and task status
//   - Send task to Runner
type Center interface {
	// Start context is for lifecycle of the Center, when context is canceled then Center exit
	Start(ctx context.Context) error

	// ReceiveMgrMsg Receive management message from MgrService, and return immediately.
	ReceiveMgrMsg(msg MgrMsg)

	GetPendingJobs() []PendingJobItem

	GetPendingTasks(jobId string) []Task
}

// Runner
// - Receive task from Center
// - Manage TaskQueue for thing
// - Do task operation (eg: invoke direct method, set shadow, notify task to thing)
// - Corresponding thing's task request (get / update)
// - Notify the Center of the task status change
type Runner interface {
	// Start context is for lifecycle of the Runner, when context is canceled then Runner exit
	Start(ctx context.Context, jobCtxGetter ctxGetter)

	// Receive task from center

	PutTasks(operation string, l []Task)
	DeleteTaskOfJob(jobId, operation string, force bool)
	CancelTaskOfJob(jobId, operation string, force bool)
	DeleteTask(taskId int64, operation string, force bool)
	CancelTask(taskId int64, operation string, force bool)

	// OnTaskChange Notify task change to Center
	OnTaskChange() <-chan TaskChangeMsg

	// GetPendingTasksOfSys get pending tasks for system operations
	// op: SysOpDirectMethod, SysOpUpdateShadow
	GetPendingTasksOfSys(op string) []Task
	// GetPendingTasksOfCustom get pending tasks for custom operations
	GetPendingTasksOfCustom() []Task
}

type MgrMsgType string

const (
	MgrTypeCreateJob  MgrMsgType = "createJob"
	MgrTypeUpdateJob  MgrMsgType = "updateJob"
	MgrTypeCancelJob  MgrMsgType = "cancelJob"
	MgrTypeDeleteJob  MgrMsgType = "deleteJob"
	MgrTypeCancelTask MgrMsgType = "cancelTask"
	MgrTypeDeleteTask MgrMsgType = "deleteTask"
)

type MgrMsg struct {
	Typ  MgrMsgType
	Data any
}

type MgrMsgCreateJob struct {
	JobContext   JobContext
	TargetConfig TargetConfig
}
type MgrMsgUpdateJob struct {
	JobId         string
	RetryConfig   RetryConfig
	TimeoutConfig TimeoutConfig
}
type MgrMsgCancelJob struct {
	JobId     string
	Operation string
	Force     bool
}
type MgrMsgDeleteJob struct {
	JobId     string
	Operation string
	Force     bool
}
type MgrMsgCancelTask struct {
	JobId     string
	TaskId    int64
	Operation string
	Force     bool
}
type MgrMsgDeleteTask struct {
	JobId     string
	TaskId    int64
	Operation string
	Force     bool
}

type ctxGetter func(jobId string) *JobContext
type JobContext struct {
	JobId string

	JobDoc    map[string]any
	Operation string

	SchedulingConfig *SchedulingConfig
	RolloutConfig    *RolloutConfig
	RetryConfig      *RetryConfig
	TimeoutConfig    *TimeoutConfig

	Status        Status
	ForceCanceled bool
	StartedAt     *int64
}

type TaskChangeMsg struct {
	JobId         string
	TaskId        int64
	ThingId       string
	Operation     string
	Status        TaskStatus
	StatusDetails StatusDetails
	Progress      uint8
}

//
// ------------------------------- JobCenter -------------------------------
//

func NewCenter(
	opt CenterOptions,
	r Repo,
	pubSub connector.PubSub,
	methodHandler shadow.MethodHandler,
	shadowSetter shadow.StateDesiredSetter,
) Center {
	jc := make(map[string]*JobContext)
	runner := NewRunner(r, pubSub, methodHandler, shadowSetter)
	p, err := ants.NewPool(centerWorkerPoolSize)
	if err != nil {
		log.Fatalf("JobCenter init pool: %v", err)
	}
	return &centerImpl{
		opt:  opt,
		repo: r,
		pool: p,

		pendingJobCh: make(chan PendingJobItem),
		pendingJobChDel: make(chan struct {
			jobId string
			force bool
		}),

		getPendingReqCh:  make(chan struct{}),
		getPendingRespCh: nil,

		runner:      runner,
		jobContexts: jc,
	}
}

type CenterOptions struct {
	CheckJobStatusInterval time.Duration
	ScheduleInterval       time.Duration
}

type PendingJobItem struct {
	Context     JobContext
	Tasks       []Task
	RolloutStat []struct {
		Time  time.Time
		Count int
	}
}
type centerImpl struct {
	ctx context.Context
	opt CenterOptions

	repo Repo
	pool *ants.Pool

	// channels for add and delete pending job
	pendingJobCh    chan PendingJobItem
	pendingJobChDel chan struct {
		jobId string
		force bool
	}

	// channels for get pending jobs
	getPendingReqCh  chan struct{}
	getPendingRespCh chan []PendingJobItem

	jcLock      sync.RWMutex
	jobContexts map[string]*JobContext // jobId->jobContext
	runner      Runner
}

func (c *centerImpl) Start(ctx context.Context) error {
	c.ctx = ctx
	c.runner.Start(ctx, c.getJobContext)
	go c.watchTaskChangeLoop()
	go c.scheduleJobLoop()

	return nil
}

func (c *centerImpl) ReceiveMgrMsg(msg MgrMsg) {
	log.Infof("JobCenter received manager message: %#v", msg)
	submit := func(doFunc func()) {
		if err := c.pool.Submit(doFunc); err != nil {
			log.Errorf("JobCenter submit job for management, msg=%#v, error=%v", msg, err)
		}
	}
	switch msg.Typ {
	case MgrTypeCreateJob:
		d := msg.Data.(MgrMsgCreateJob)
		submit(func() {
			jc := c.setJobContext(d.JobContext.JobId, d.JobContext)
			if l, err := c.createJob(d); err != nil {
				log.Errorf("JobCenter create tasks: %v, jobId=%q", err, d.JobContext.JobId)
			} else {
				log.Infof("JobCenter created tasks, jobId=%q, count=%d", d.JobContext.JobId, len(l))
				c.addPendingJob(PendingJobItem{Context: *jc, Tasks: l})
			}
		})
	case MgrTypeUpdateJob:
		d := msg.Data.(MgrMsgUpdateJob)
		if jc := c.getJobContext(d.JobId); jc != nil {
			jc.RetryConfig = &d.RetryConfig
			jc.TimeoutConfig = &d.TimeoutConfig
			c.setJobContext(d.JobId, *jc)
		}
	case MgrTypeCancelJob:
		d := msg.Data.(MgrMsgCancelJob)
		var jc *JobContext
		if jc = c.getJobContext(d.JobId); jc != nil {
			jc.Status = StatusCanceled
			jc.ForceCanceled = d.Force
			c.setJobContext(d.JobId, *jc)
		}
		if d.Force {
			c.removePendingJob(d.JobId, d.Force)
			c.removeJobContext(d.JobId)
		}
		submit(func() {
			c.runner.CancelTaskOfJob(d.JobId, d.Operation, d.Force)
		})
		if err := c.repo.UpdateJob(c.ctx, d.JobId, map[string]any{
			"status":       StatusCanceled,
			"completed_at": time.Now(),
		}); err != nil {
			log.Errorf("JobCenter update job canceled, jobId=%q, error: %v", d.JobId, err)
		}
	case MgrTypeDeleteJob:
		d := msg.Data.(MgrMsgDeleteJob)
		if jc := c.getJobContext(d.JobId); jc != nil {
			jc.Status = StatusRemoving
			c.setJobContext(d.JobId, *jc)
		}
		c.removePendingJob(d.JobId, d.Force)
		submit(func() {
			c.runner.DeleteTaskOfJob(d.JobId, d.Operation, d.Force)
		})
	case MgrTypeCancelTask:
		d := msg.Data.(MgrMsgCancelTask)
		submit(func() {
			c.runner.DeleteTask(d.TaskId, d.Operation, d.Force)
		})
	case MgrTypeDeleteTask:
		d := msg.Data.(MgrMsgCancelTask)
		submit(func() {
			c.runner.DeleteTask(d.TaskId, d.Operation, d.Force)
		})
	default:
		log.Errorf("unknown job manage message type: %q", msg.Typ)
	}
}

func (c *centerImpl) GetPendingJobs() []PendingJobItem {
	c.getPendingRespCh = make(chan []PendingJobItem, 1)
	defer func() {
		c.getPendingRespCh = nil
	}()
	tm := time.After(time.Second)

f:
	for {
		select {
		case <-c.ctx.Done():
			break f
		case <-tm:
			log.Errorf("JobCenter get pending job timeout")
			break f
		case c.getPendingReqCh <- struct{}{}:
			continue
		case r := <-c.getPendingRespCh:
			return r
		}
	}
	return []PendingJobItem{}
}

func (c *centerImpl) GetPendingTasks(jobId string) []Task {
	l := c.GetPendingJobs()
	for _, j := range l {
		if j.Context.JobId == jobId {
			return j.Tasks
		}
	}
	return []Task{}
}

func (c *centerImpl) addPendingJob(j PendingJobItem) {
	c.pendingJobCh <- j
}
func (c *centerImpl) removePendingJob(jobId string, force bool) {
	c.pendingJobChDel <- struct {
		jobId string
		force bool
	}{jobId: jobId, force: force}
}

func (c *centerImpl) scheduleJobLoop() {
	tk := time.NewTicker(c.opt.ScheduleInterval)
	var pendingJobs []*PendingJobItem
	delPendingAt := func(index int) {
		pendingJobs = append(pendingJobs[:index], pendingJobs[index+1:]...)
	}

	// preload pending jobs from db

	//l, err := c.repo.GetPendingJobs(c.ctx)
	//if err != nil {
	//	log.Fatalf("JobCenter get pending jobs: %v", err)
	//}
	//for _, j := range l {
	//	if len(j.Tasks) == 0 {
	//		log.Warnf("JobCenter job has no pending task, to terminate it, jobId=%q", j.JobId)
	//		// TODO finish job by current job status
	//		continue
	//	}
	//	if d, err := toDetail(j, []TaskStatusCount{}); err != nil {
	//		log.Fatalf("JobCenter convert entity to Detail, jobId=%q, error: %v", j.JobId, err)
	//	} else {
	//		p := PendingJobItem{
	//			Context: JobContext{
	//				JobId: d.JobId, Operation: d.Operation, JobDoc: d.JobDoc,
	//				SchedulingConfig: d.SchedulingConfig, RolloutConfig: d.RolloutConfig,
	//				RetryConfig: d.RetryConfig, TimeoutConfig: d.TimeoutConfig,
	//				Status: d.Status, StartedAt: d.StartedAt,
	//			},
	//			Tasks: toTasks(j.Tasks),
	//		}
	//		pendingJobs = append(pendingJobs, &p)
	//	}
	//}

	for {
		select {
		case <-c.ctx.Done():
		case i := <-c.pendingJobCh:
			pendingJobs = append(pendingJobs, &i)
		case del := <-c.pendingJobChDel:
			for i, j := range pendingJobs {
				if j.Context.JobId == del.jobId {
					if del.force {
						delPendingAt(i)
					} else {
						var tl []Task
						// remove unscheduled task
						for _, t := range j.Tasks {
							if isTaskOngoing(t.Status) {
								tl = append(tl, t)
							}
						}
						j.Tasks = tl
						if len(j.Tasks) == 0 {
							delPendingAt(i)
						}
					}
				}
			}
			continue
		case <-c.getPendingReqCh:
			if c.getPendingRespCh != nil {
				_ = c.pool.Submit(func() {
					var cp []PendingJobItem
					for _, j := range pendingJobs {
						cp = append(cp, *j)
					}
					c.getPendingRespCh <- cp
				})
			}
		case <-tk.C:
		}
		l := len(pendingJobs)
		for i := 0; i < l; i++ {
			v := pendingJobs[i]
			jc := &v.Context
			tasks := v.Tasks

			globalJc := c.getJobContext(jc.JobId)
			if globalJc != nil {
				jc = globalJc
			} else {
				c.setJobContext(jc.JobId, *jc)
			}
			if jc.SchedulingConfig == nil || jc.SchedulingConfig.StartTime.Before(time.Now()) {
				// update status
				if jc.Status == StatusWaiting {
					if err := c.repo.UpdateJob(
						c.ctx, jc.JobId,
						map[string]any{"status": StatusInProgress, "started_at": time.Now()},
					); err != nil {
						log.Errorf("JobCenter update job status, job=%q, status=%q, error: %v",
							jc.JobId, StatusInProgress, err)
						continue
					}
					jc.Status = StatusInProgress
				}

				// filter tasks when job is to terminal status
				if isJobToTerminal(jc.Status) {
					var l []Task
					for _, t := range v.Tasks {
						if isTaskOngoing(t.Status) {
							l = append(l, t)
						}
					}
					v.Tasks = l
				}

				// rollout count

				var rolloutTasks []Task
				rolloutCount := len(tasks)
				if jc.RolloutConfig != nil && jc.RolloutConfig.MaxPerMinute > 0 {
					rolloutCount = getNextRolloutCount(jc.RolloutConfig.MaxPerMinute, *v)
				}
				rolloutTasks = v.Tasks[:rolloutCount]
				v.Tasks = v.Tasks[rolloutCount:]

				log.Infof("JobCenter scheduled jobId=%q, taskCount=%d, rolloutCount=%d",
					jc.JobId, len(tasks), len(rolloutTasks))
				v.RolloutStat = append(v.RolloutStat, struct {
					Time  time.Time
					Count int
				}{Time: time.Now(), Count: len(rolloutTasks)})

				// do rollout
				c.runner.PutTasks(jc.Operation, rolloutTasks)

				// delete completed job
				if len(v.Tasks) == 0 {
					delPendingAt(i)
					l = len(pendingJobs)
				}
			} else if jc.SchedulingConfig.EndTime != nil && jc.SchedulingConfig.EndTime.Before(time.Now()) {
				log.Errorf("JobCenter schedule job which is end, jobId=%s, endTime=%s",
					jc.JobId, jc.SchedulingConfig.EndTime)
				delPendingAt(i)
				l = len(pendingJobs)
			} else {
				// filter tasks when job is to terminal status
				if isJobToTerminal(jc.Status) {
					var l []Task
					for _, t := range v.Tasks {
						if isTaskOngoing(t.Status) {
							l = append(l, t)
						}
					}
					v.Tasks = l
				}
				// delete completed job
				if len(v.Tasks) == 0 {
					delPendingAt(i)
					c.removeJobContext(jc.JobId)
					l = len(pendingJobs)
				}
			}
		}
	}
}

func (c *centerImpl) watchTaskChangeLoop() {
	tcCh := c.runner.OnTaskChange()
	pendingCheckJobs := map[string]struct{}{}
	checkJobTick := time.NewTicker(c.opt.CheckJobStatusInterval)
	for {
		var msg TaskChangeMsg
		select {
		case <-c.ctx.Done():
			return
		case <-checkJobTick.C:
			for k := range pendingCheckJobs {
				if c.checkJobFinish(k) {
					delete(pendingCheckJobs, k)
				}
			}
		case msg = <-tcCh:
			log.Infof("JobCenter watched task change, jobId=%q, taskId=%d, thingId=%q, "+
				"status=%q, progress=%d, statusDetails=%v",
				msg.JobId, msg.TaskId, msg.ThingId, msg.Status, msg.Progress, msg.StatusDetails)
		}
		if isTaskTerminal(msg.Status) {
			pendingCheckJobs[msg.JobId] = struct{}{}
		}

		// TODO: more for task change
	}
}

func (c *centerImpl) checkJobFinish(jobId string) (finished bool) {
	defer func() {
		if finished {
			c.removePendingJob(jobId, true)
			c.removeJobContext(jobId)
		}
	}()
	res, err := c.repo.CountTaskStatus(c.ctx, jobId)
	if err != nil {
		log.Errorf("JobCenter get task status count error: %v", err)
		return
	}
	for _, cs := range res {
		if !isTaskTerminal(cs.Status) {
			return false
		}
	}
	j, err := c.repo.GetJob(c.ctx, jobId)
	if err != nil {
		log.Errorf("JobCenter get job jobId=%q, %v", jobId, err)
		return
	}
	if j == nil {
		log.Errorf("JobCenter get job nil jobId=%q", jobId)
		finished = true
		return
	}
	var st Status
	switch j.Status {
	case StatusCanceling:
		st = StatusCanceled
	case StatusRemoving:
		if _, err := c.repo.DeleteJob(c.ctx, j.JobId, true); err != nil {
			finished = true
			return
		} else {
			log.Errorf("JobCenter delete job, jobId=%q, error: %v", j.JobId, err)
			finished = false
			return
		}
	case StatusWaiting, StatusInProgress:
		st = StatusCompleted
	default:
		log.Errorf("JobCenter unexpected job status when check, jobId=%q, status=%q", j.JobId, j.Status)
		finished = true
		return
	}
	err = c.repo.UpdateJob(c.ctx, jobId, map[string]any{
		"status":       st,
		"completed_at": time.Now(),
	})
	if err != nil {
		log.Errorf("JobCenter update job finish, jobId=%q, %v", jobId, err)
	} else {
		log.Infof("JobCenter job finished, jobId=%q, status=%q", jobId, st)
	}
	finished = true
	return
}

func (c *centerImpl) getJobContext(jobId string) *JobContext {
	c.jcLock.RLock()
	defer c.jcLock.RUnlock()
	return c.jobContexts[jobId]
}
func (c *centerImpl) setJobContext(jobId string, jobCtx JobContext) *JobContext {
	c.jcLock.Lock()
	defer c.jcLock.Unlock()
	c.jobContexts[jobId] = &jobCtx
	log.Debugf("JobCenter set job context jobId=%q", jobId)
	return &jobCtx
}
func (c *centerImpl) removeJobContext(jobId string) {
	c.jcLock.Lock()
	defer c.jcLock.Unlock()
	delete(c.jobContexts, jobId)
	log.Debugf("JobCenter remove job context jobId=%q", jobId)
}

func (c *centerImpl) createJob(m MgrMsgCreateJob) ([]Task, error) {
	jobId := m.JobContext.JobId
	l := toTaskEntities(jobId, m.JobContext.Operation, m.TargetConfig)
	if rl, err := c.repo.CreateTasks(c.ctx, l); err != nil {
		return []Task{}, err
	} else {
		return toTasks(rl), nil
	}
}

var _ Center = &centerImpl{}

//
// ------------------------------- JobRunner -------------------------------
//

func NewRunner(
	repo Repo,
	pubSub connector.PubSub,
	methodHandler shadow.MethodHandler,
	shadowSetter shadow.StateDesiredSetter,
) Runner {
	ttq := make(map[string]TaskQueue)
	p, err := ants.NewPool(runnerWorkerPoolSize, ants.WithNonblocking(true))
	if err != nil {
		log.Fatalf("JobRunner init pool: %v", err)
	}
	return &runnerImpl{
		repo: repo,

		pool:            p,
		thingTaskQueues: ttq,

		// channels for task change
		innerTaskChangeCh: make(chan TaskChangeMsg),
		outTaskChangeCh:   make(chan TaskChangeMsg),

		// channels for direct method task
		directMethodCh:       make(chan []Task),
		directMethodDeleteCh: make(chan []int64),

		// channels for get tasks
		getPendingTasksOfCustomReqCh:        make(chan struct{}),
		getPendingTasksOfCustomRespCh:       nil,
		getPendingTasksOfDirectMethodReqCh:  make(chan struct{}),
		getPendingTasksOfDirectMethodRespCh: nil,
		getPendingTasksOfUpdateShadowReqCh:  make(chan struct{}),
		getPendingTasksOfUpdateShadowRespCh: nil,

		// for actions

		pubSub:        pubSub,
		methodHandler: methodHandler,
		shadowSetter:  shadowSetter,
	}
}

type runnerImpl struct {
	ctx context.Context

	repo Repo

	jcGetter ctxGetter
	pool     *ants.Pool

	innerTaskChangeCh chan TaskChangeMsg
	outTaskChangeCh   chan TaskChangeMsg

	directMethodCh       chan []Task
	directMethodDeleteCh chan []int64

	// channels for get tasks
	getPendingTasksOfCustomReqCh        chan struct{}
	getPendingTasksOfCustomRespCh       chan []Task
	getPendingTasksOfDirectMethodReqCh  chan struct{}
	getPendingTasksOfDirectMethodRespCh chan []Task
	getPendingTasksOfUpdateShadowReqCh  chan struct{}
	getPendingTasksOfUpdateShadowRespCh chan []Task

	thingTaskQueues map[string]TaskQueue // thingId->[]Task, for general task

	pubSub        connector.PubSub
	methodHandler shadow.MethodHandler
	shadowSetter  shadow.StateDesiredSetter
}

var _ Runner = &runnerImpl{}

func (r *runnerImpl) Start(ctx context.Context, jcGetter ctxGetter) {
	r.ctx = ctx
	r.jcGetter = jcGetter
	go r.watchTaskChangeLoop()
	go r.directMethodLoop(r.directMethodCh, r.directMethodDeleteCh)
}

func (r *runnerImpl) OnTaskChange() <-chan TaskChangeMsg {
	return r.outTaskChangeCh
}

func (r *runnerImpl) PutTasks(operation string, l []Task) {
	switch operation {
	case SysOpDirectMethod:
		r.directMethodCh <- l
	case SysOpUpdateShadow:
		// TODO set shadow
	default:
		// TODO custom
	}
}

func (r *runnerImpl) GetPendingTasksOfSys(op string) []Task {
	switch op {
	case SysOpDirectMethod:
		r.getPendingTasksOfDirectMethodRespCh = make(chan []Task, 1)
		defer func() {
			r.getPendingTasksOfDirectMethodRespCh = nil
		}()
		r.getPendingTasksOfDirectMethodReqCh <- struct{}{}
		return <-r.getPendingTasksOfDirectMethodRespCh
	case SysOpUpdateShadow:
		r.getPendingTasksOfUpdateShadowRespCh = make(chan []Task, 1)
		defer func() {
			r.getPendingTasksOfUpdateShadowRespCh = nil
		}()
		r.getPendingTasksOfUpdateShadowReqCh <- struct{}{}
		return <-r.getPendingTasksOfUpdateShadowRespCh
	default:
		return []Task{}
	}
}

func (r *runnerImpl) GetPendingTasksOfCustom() []Task {
	r.getPendingTasksOfCustomRespCh = make(chan []Task, 1)
	defer func() {
		r.getPendingTasksOfCustomRespCh = nil
	}()
	r.getPendingTasksOfCustomReqCh <- struct{}{}
	return <-r.getPendingTasksOfCustomRespCh
}

func (r *runnerImpl) DeleteTaskOfJob(jobId, operation string, force bool) {
	tl, err := r.repo.GetTasksOfJob(r.ctx, jobId, []TaskStatus{TaskCanceled})
	if err != nil {
		log.Errorf("JobRunner get tasks of job, jobId=%s, error: %v", jobId, err)
		return
	}
	switch operation {
	case SysOpDirectMethod:
		var taskIds []int64
		for _, t := range tl {
			taskIds = append(taskIds, t.TaskId)
		}
		r.directMethodDeleteCh <- taskIds
	case SysOpUpdateShadow:
	default:
	}
}

func (r *runnerImpl) CancelTaskOfJob(jobId, operation string, force bool) {
	tl, err := r.repo.GetTasksOfJob(r.ctx, jobId, []TaskStatus{TaskCanceled})
	if err != nil {
		log.Errorf("JobRunner get tasks of job, jobId=%s, error: %v", jobId, err)
		return
	}
	switch operation {
	case SysOpDirectMethod:
		var taskIds []int64
		for _, t := range tl {
			taskIds = append(taskIds, t.TaskId)
		}
		r.directMethodDeleteCh <- taskIds
		log.Debugf("JobRunner sent msg for delete tasks of direct method: %v", taskIds)
	case SysOpUpdateShadow:
	default:
	}
}

func (r *runnerImpl) DeleteTask(taskId int64, operation string, force bool) {
	switch operation {
	case SysOpDirectMethod:
		r.directMethodDeleteCh <- []int64{taskId}
	case SysOpUpdateShadow:
	default:
	}
}

func (r *runnerImpl) CancelTask(taskId int64, operation string, force bool) {
	switch operation {
	case SysOpDirectMethod:
		r.directMethodDeleteCh <- []int64{taskId}
	case SysOpUpdateShadow:
	default:
	}
}

func (r *runnerImpl) watchTaskChangeLoop() {
	for {
		select {
		case <-r.ctx.Done():
			log.Debug("JobRunner task change watcher exit cause context closed")
		case chMsg := <-r.innerTaskChangeCh:
			r.updateTaskStatus(chMsg)
			r.outTaskChangeCh <- chMsg

			if isTaskTerminal(chMsg.Status) {
				if chMsg.Operation != SysOpDirectMethod && chMsg.Operation != SysOpUpdateShadow {
					// TODO: update thing task queue
				}
			}
		}
	}
}

func (r *runnerImpl) updateTaskStatus(msg TaskChangeMsg) {
	sdBuf, err := json.Marshal(msg.StatusDetails)
	if err != nil {
		log.Errorf("JobRunner update task status, unexpected marshal statusDetails=%v, jobId=%q, taskId=%d, error: %v",
			msg.StatusDetails, msg.JobId, msg.TaskId, err)
	}
	err = r.repo.ExecWithTx(func(txRepo Repo) error {
		t, err := txRepo.GetTask(r.ctx, msg.TaskId)
		if err != nil {
			return err
		}
		if t == nil {
			return errors.New("task not found")
		}
		if isTaskTerminal(t.Status) {
			return fmt.Errorf("task is terminal at status=%q", t.Status)
		}
		err = txRepo.UpdateTask(r.ctx, msg.TaskId, map[string]any{
			"status":         msg.Status,
			"progress":       msg.Progress,
			"status_details": sdBuf,
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Errorf("JobRunner update task status, jobId=%q, taskId=%d, status=%q, error: %v",
			msg.JobId, msg.TaskId, msg.Status, err)
	}
	log.Debugf("JobRunner update task status, jobId=%q, taskId=%d, status=%q, progress=%v",
		msg.JobId, msg.TaskId, msg.Status, msg.Progress)
}

func (r *runnerImpl) directMethodLoop(ch <-chan []Task, delCh <-chan []int64) {
	defer func() {
		log.Info("JobRunner direct method loop exit")
	}()
	concurrentOnTick := 10
	// The task queue is only used in this go routine for lock-free
	q := NewTaskQueue()
	tk := time.NewTicker(time.Millisecond * 50)
	for {
		select {
		case <-r.ctx.Done():
			log.Debugf("JobRunner direct method watcher exit cause context closed")
			return
		case tl := <-ch:
			for _, t := range tl {
				st := t
				log.Debugf("JobRunner push task %d", st.TaskId)
				q.Push(&st)
			}
			continue
		case dl := <-delCh:
			for _, id := range dl {
				_ = q.RemoveById(id)
			}
			continue
		case <-r.getPendingTasksOfDirectMethodReqCh:
			_ = r.pool.Submit(func() {
				r.getPendingTasksOfDirectMethodRespCh <- q.GetTasks()
			})
		case <-tk.C:
			// do task below
		}

		c := 0
		for q.Size() > 0 && c < concurrentOnTick {
			c++
			t := q.Pop()
			jc := r.jcGetter(t.JobId)
			if jc == nil {
				// should never happen
				log.Warnf("JobRunner job context is nil, maybe deleted jobId=%s", t.JobId)
				continue
			}
			if isJobToTerminal(jc.Status) {
				log.Infof("JobRunner job is going to terminal status %q, give up task %d for thing %q",
					jc.Status, t.TaskId, t.ThingId)
				continue
			}

			var req InvokeDirectMethodReq
			if jc.JobDoc == nil {
				log.Fatalf("JobRunner unexpected job doc is nil for invoke direct method! jobId=%q, jobDoc=%v",
					jc.JobId, jc.JobDoc)
				continue
			}

			jBuf, err := json.Marshal(jc.JobDoc)
			if err != nil {
				log.Fatalf("JobRunner unexpected job doc is nil for invoke direct method! jobId=%q, jobDoc=%v",
					jc.JobId, jc.JobDoc)
				continue
			}
			if err := json.Unmarshal(jBuf, &req); err != nil {
				// job doc should be checked before job created
				log.Fatalf("JobRunner unexpected job doc for invoke direct method! jobId=%q, jobDoc=%v",
					jc.JobId, jc.JobDoc)
				continue
			}
			err = r.pool.Submit(func() {
				if re, err := r.doInvokeDirectMethod(jc.JobId, t.TaskId, t.ThingId, req); err != nil {
					log.Errorf("JobRunner do invoke direct method, jobId=%q taskId=%d thingId=%s : %v",
						jc.JobId, t.TaskId, t.ThingId, err)
				} else {
					// notify result
					r.innerTaskChangeCh <- re
				}
			})
			if err != nil {
				log.Warnf("JobRunner direct method task submit error: %v", err)
				q.Push(t)
			} else {
				log.Infof("JobRunner direct method task submit success, jobId=%q, taskId=%d, thingId=%q",
					jc.JobId, t.TaskId, t.ThingId)
				r.innerTaskChangeCh <- TaskChangeMsg{JobId: jc.JobId, TaskId: t.TaskId, ThingId: t.ThingId, Status: TaskSent}
			}
		}
	}
}

// ------------------------- helper func -------------------------

func isJobToTerminal(s Status) bool {
	if s == StatusCanceling || s == StatusCanceled || s == StatusRemoving || s == StatusCompleted {
		return true
	}
	return false
}

func isTaskTerminal(s TaskStatus) bool {
	return s == TaskFailed ||
		s == TaskSucceeded ||
		s == TaskTimeOut ||
		s == TaskRejected ||
		s == TaskCanceled
}

func isTaskOnInit(s TaskStatus) bool {
	return s == TaskQueued
}

func isTaskOngoing(s TaskStatus) bool {
	return s == TaskSent || s == TaskInProgress
}

func getNextRolloutCount(maxCountPerMinute int, p PendingJobItem) int {
	minuteAgo := time.Now().Add(-time.Minute)
	curMinuteCount := 0
	for _, r := range p.RolloutStat {
		if r.Time.After(minuteAgo) {
			curMinuteCount += r.Count
		}
	}
	if curMinuteCount >= maxCountPerMinute {
		return 0
	}
	maxCur := maxCountPerMinute - curMinuteCount
	if maxCur > len(p.Tasks) {
		maxCur = len(p.Tasks)
	}
	return maxCur
}
