package job

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

// Scheduler manage the jobs and tasks according to configs
// When the tasks state change, the task queue for Things to run also change
//                                   ┌──────────────────────────┐
//                                   │         job center       │
//                                   │                          │
//                                   │   ┌───────────────────┐  │
//                                   │   │     scheduler     │  │
//                                   │   │                   │  │
//  manage (job/task manage) ───────►│   │       timing      │  ├──────► database(save state)
//                                   │   │   retry, timeout  │  │
//                                   │   └───────────────────┘  │
//                                   │             ▼            │
//                                   │   ┌───────────────────┐  │
//                                   │   │    state trans    │  │
//                                   │   └───────────────────┘  │
//                                   │             ▼            │
//                                   │   ┌───────────────────┐  │
//                                   │   │     task queue    │  │
//       Device(get/report)  ───────►│   └───────────────────┘  ├──────► Action(notify-thing/direct-method/update-shadow)
//                                   │                          │
//                                   └──────────────────────────┘

const centerWorkerPoolSize = 100
const runnerWorkerPoolSize = 500

// Center Entrance to task operation management
//   - Receive management messages from the upper layer
//   - Subscribe thing's task request (get / update)
//   - Schedule job and task (according to schedule config , timeout config ...)
//   - Manage job and task status
//   - Send task to Runner
type Center interface {
	// Start context is for lifecycle of the Center, when context is canceled then Center exit
	Start(ctx context.Context) error

	// ReceiveMgrMsg Receive management message from MgrService, and return immediately.
	ReceiveMgrMsg(msg MgrMsg)
}

// Runner
// - Receive task from Center
// - Manage TaskQueue for thing
// - Do task operation (eg: invoke direct method, set shadow, notify task to thing)
// - Notify the Center of the task status change
type Runner interface {
	// Start context is for lifecycle of the Runner, when context is canceled then Runner exit
	Start(ctx context.Context, jobCtxGetter ctxGetter)

	// OnTaskChange Notify task change
	OnTaskChange() <-chan TaskChangeMsg

	PutTasks(operation string, l []Task)

	DeleteTasks(taskIds []int64)

	CancelTasks(l []Task)
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

type ctxGetter func(jobId string) *JobContext
type JobContext struct {
	JobId string `json:"jobId"`

	JobDoc    map[string]any `json:"jobDoc"`
	Operation string         `json:"operation"`

	SchedulingConfig *SchedulingConfig `json:"schedulingConfig"`
	RetryConfig      *RetryConfig      `json:"retryConfig"`
	TimeoutConfig    *TimeoutConfig    `json:"timeoutConfig"`

	Status    Status `json:"status"`
	StartedAt *int64 `json:"startedAt"`
}

type TaskChangeMsg struct {
	JobId         string
	TaskId        int64
	ThingId       string
	Operation     string
	Status        TaskStatus    `json:"status"`
	StatusDetails StatusDetails `json:"statusDetails"`
	Progress      uint8         `json:"progress"`
}

//
// ------------------------------- JobCenter -------------------------------
//

func NewCenter(
	ctx context.Context,
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
		ctx:  ctx,
		opt:  opt,
		repo: r,
		pool: p,

		runner: runner,
		jc:     jc,
	}
}

type CenterOptions struct {
	CheckJobStatusInterval time.Duration
}

type centerImpl struct {
	ctx context.Context
	opt CenterOptions

	repo Repo
	pool *ants.Pool

	jcLock sync.RWMutex
	jc     map[string]*JobContext // jobId->jobContext
	runner Runner
}

func (c *centerImpl) Start(ctx context.Context) error {

	c.runner.Start(c.ctx, c.getJobContext)
	go c.watchTaskChange()

	// TODO load jobs; update pending jobs state by scanning tasks

	return nil
}

func (c *centerImpl) ReceiveMgrMsg(msg MgrMsg) {
	switch msg.Typ {
	case MgrTypeCreateJob:
		data := msg.Data.(MgrMsgCreateJob)
		if err := c.pool.Submit(func() {
			jc := c.setJobContext(data.JobContext.JobId, data.JobContext)
			if l, err := c.createJob(data); err != nil {
				log.Errorf("JobCenter create tasks: %v, jobId=%q", err, data.JobContext.JobId)
			} else {
				log.Infof("JobCenter created tasks, jobId=%q, count=%d", data.JobContext.JobId, len(l))
				c.scheduleJob(jc, l)
			}
		}); err != nil {
			log.Errorf("JobCenter create job error:%v", err)
		}
	case MgrTypeUpdateJob:
	case MgrTypeCancelJob:
	case MgrTypeDeleteJob:

	case MgrTypeCancelTask:
	case MgrTypeDeleteTask:
	default:
		log.Errorf("unknown job manage message type: %q", msg.Typ)
	}
}

func (c *centerImpl) watchTaskChange() {
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
		// TODO: more for task change
		if isTaskTerminal(msg.Status) {
			pendingCheckJobs[msg.JobId] = struct{}{}
		}
	}
}

func (c *centerImpl) checkJobFinish(jobId string) bool {
	res, err := c.repo.CountTaskStatus(c.ctx, jobId)
	if err != nil {
		log.Errorf("JobCenter get task status count error: %v", err)
		return false
	}
	for _, cs := range res {
		if !isTaskTerminal(cs.Status) {
			return false
		}
	}
	j, err := c.repo.GetJob(c.ctx, jobId)
	if err != nil {
		log.Errorf("JobCenter get job jobId=%q, %v", jobId, err)
		return false
	}
	if j == nil {
		log.Errorf("JobCenter get job nil jobId=%q", jobId)
		return false
	}
	var st Status
	switch j.Status {
	case StatusCanceling:
		st = StatusCanceled
	case StatusRemoving:
		// delete job
		return true
	case StatusWaiting, StatusInProgress:
		st = StatusCompleted
	default:
		log.Errorf("JobCenter unexpected job status when check, jobId=%q, status=%q", j.JobId, j.Status)
		return true
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
	return true
}

func (c *centerImpl) scheduleJob(jc *JobContext, tasks []Task) {
	if jc.SchedulingConfig == nil || jc.SchedulingConfig.StartTime.Before(time.Now()) {
		c.repo.UpdateJob(c.ctx, jc.JobId, map[string]any{"status": StatusInProgress, "started_at": time.Now()})
		log.Infof("JobCenter scheduled jobId=%q, taskCount=%d", jc.JobId, len(tasks))
		c.runner.PutTasks(jc.Operation, tasks)
	} else {
		// TODO: for schedule future
	}
}

func (c *centerImpl) getJobContext(jobId string) *JobContext {
	c.jcLock.RLock()
	defer c.jcLock.RUnlock()
	return c.jc[jobId]
}

func (c *centerImpl) setJobContext(jobId string, jobCtx JobContext) *JobContext {
	c.jcLock.Lock()
	defer c.jcLock.Unlock()
	c.jc[jobId] = &jobCtx
	return &jobCtx
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

		// channels

		innerTaskChangeCh: make(chan TaskChangeMsg),
		outTaskChangeCh:   make(chan TaskChangeMsg),
		directMethodCh:    make(chan []Task),

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
	directMethodCh    chan []Task

	thingTaskQueues map[string]TaskQueue // thingId->[]Task, for general task

	pubSub        connector.PubSub
	methodHandler shadow.MethodHandler
	shadowSetter  shadow.StateDesiredSetter
}

var _ Runner = &runnerImpl{}

func (r *runnerImpl) Start(ctx context.Context, jcGetter ctxGetter) {
	r.ctx = ctx
	r.jcGetter = jcGetter
	go r.watchTaskChange()
	go r.directMethodLoop(r.directMethodCh)
}

func (r *runnerImpl) OnTaskChange() <-chan TaskChangeMsg {
	return r.outTaskChangeCh
}

func (r *runnerImpl) PutTasks(operation string, l []Task) {
	switch operation {
	case SysOpDirectMethod:
		// r.directMethodTaskQueue.Push(&t)
		//r.directMethodTaskQueue
		r.directMethodCh <- l
	case SysOpUpdateShadow:
		// TODO set shadow
	default:
		// TODO custom
	}
}

func (r *runnerImpl) DeleteTasks(taskIds []int64) {
	//TODO implement me
	panic("implement me")
}

func (r *runnerImpl) CancelTasks(l []Task) {
	//TODO implement me
	panic("implement me")
}

func (r *runnerImpl) watchTaskChange() {
	for {
		select {
		case <-r.ctx.Done():
			log.Debug("JobRunner task change watcher exit cause context closed")
		case chMsg := <-r.innerTaskChangeCh:
			// TODO: update task queue

			// if isTaskTerminal(chMsg.Status) {
			// 	if chMsg.Operation != SysOpDirectMethod && chMsg.Operation != SysOpUpdateShadow {
			// 	}
			// }
			r.updateTaskStatus(chMsg)

			r.outTaskChangeCh <- chMsg
		}
	}
}

func (r *runnerImpl) updateTaskStatus(msg TaskChangeMsg) {
	sdBuf, err := json.Marshal(msg.StatusDetails)
	if err != nil {
		log.Errorf("JobRunner unexpected marshal StatusDetails, jobId=%q, taskId=%d, error: %v",
			msg.JobId, msg.TaskId, err)
	}
	err = r.repo.UpdateTask(r.ctx, msg.TaskId, map[string]any{
		"status":         msg.Status,
		"progress":       msg.Progress,
		"status_details": sdBuf,
	})
	if err != nil {
		log.Errorf("JobRunner update task status failed, jobId=%q, taskId=%d error: %v",
			msg.JobId, msg.TaskId, err)
	}
}

func (r *runnerImpl) directMethodLoop(ch <-chan []Task) {
	tickConcurrent := 10
	// The task queue is only used in this go routine for lock-free
	q := NewTaskQueue()
	tk := time.NewTicker(time.Microsecond * 10)
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
		case <-tk.C:
		}

		c := 0
		for q.Size() > 0 && c < tickConcurrent {
			c++
			t := q.Pop()
			jc := r.jcGetter(t.JobId)
			if jc == nil {
				// should never happen
				log.Fatalf("JobRunner unexpected job context is nil, jobId=%s", t.JobId)
				return
			}
			var req InvokeDirectMethodReq
			if jc.JobDoc == nil {
				log.Fatalf("JobRunner unexpected job doc is nil for invoke direct method! jobId=%q, jobDoc=%v",
					jc.JobId, jc.JobDoc)
				return
			}

			jBuf, err := json.Marshal(jc.JobDoc)
			if err != nil {
				log.Fatalf("JobRunner unexpected job doc is nil for invoke direct method! jobId=%q, jobDoc=%v",
					jc.JobId, jc.JobDoc)
				return
			}
			if err := json.Unmarshal(jBuf, &req); err != nil {
				// job doc should be checked before job created
				log.Fatalf("JobRunner unexpected job doc for invoke direct method! jobId=%q, jobDoc=%v",
					jc.JobId, jc.JobDoc)
				return
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
			}
		}
	}
}

// ------------------------- helper func -------------------------

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
