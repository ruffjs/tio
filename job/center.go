package job

import (
	"context"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

func NewCenter(
	opt CenterOptions,
	r Repo,
	pubSub connector.PubSub, conn connector.ConnectChecker,
	methodHandler shadow.MethodHandler,
	shadowSetter shadow.StateDesiredSetter,
) Center {
	jc := make(map[string]*JobContext)
	runner := NewRunner(r, pubSub, conn, methodHandler, shadowSetter)
	p, err := ants.NewPool(centerWorkerPoolSize)
	if err != nil {
		log.Fatalf("JobCenter init pool: %v", err)
	}
	return &centerImpl{
		opt:  opt,
		repo: r,
		pool: p,

		pendingJobCh:       make(chan PendingJobItem),
		pendingJobChDel:    make(chan removeJobMsg),
		updateJobContextCh: make(chan updateJobMsg),

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

type removeJobMsg struct {
	jobId string
	typ   MgrMsgType
	force bool
}

type updateJobMsg struct {
	jobId         string
	RetryConfig   *RetryConfig
	TimeoutConfig *TimeoutConfig
}

type centerImpl struct {
	ctx context.Context
	opt CenterOptions

	repo Repo
	pool *ants.Pool

	// channels for add and delete pending job
	pendingJobCh    chan PendingJobItem
	pendingJobChDel chan removeJobMsg

	// channels for get pending jobs
	getPendingReqCh    chan struct{}
	getPendingRespCh   chan []PendingJobItem
	updateJobContextCh chan updateJobMsg

	jcLock      sync.RWMutex
	jobContexts map[string]*JobContext // jobId->jobContext
	runner      Runner
}

func (c *centerImpl) Start(ctx context.Context) error {
	c.ctx = ctx
	c.runner.Start(ctx, c.getJobContext)
	go c.watchTaskChangeLoop()

	// preload pending jobs from db
	l := c.preloadPendingJobs()
	go c.rolloutLoop(l)

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
			if l, err := c.createJob(d); err != nil {
				log.Errorf("JobCenter create tasks: %v, jobId=%q", err, d.JobContext.JobId)
			} else {
				log.Infof("JobCenter created tasks, jobId=%q, count=%d", d.JobContext.JobId, len(l))
				c.addPendingJob(PendingJobItem{Context: d.JobContext, Tasks: l})
			}
		})
	case MgrTypeUpdateJob:
		d := msg.Data.(MgrMsgUpdateJob)
		ujMsg := updateJobMsg{
			jobId:         d.JobId,
			TimeoutConfig: d.TimeoutConfig,
			RetryConfig:   d.RetryConfig,
		}
		c.updateJobContextCh <- ujMsg
	case MgrTypeCancelJob:
		d := msg.Data.(MgrMsgCancelJob)
		if d.Force {
			c.removeJobContext(d.JobId)
		}
		c.removePendingJob(removeJobMsg{jobId: d.JobId, force: d.Force, typ: MgrTypeCancelJob})
		submit(func() {
			c.runner.CancelTaskOfJob(d.JobId, d.Operation, d.Force)
		})
		if err := c.repo.UpdateJob(c.ctx, d.JobId, map[string]any{
			"status":         StatusCanceled,
			"force_canceled": d.Force,
			"completed_at":   time.Now(),
		}); err != nil {
			log.Errorf("JobCenter update job canceled, jobId=%q, error: %v", d.JobId, err)
		}
	case MgrTypeDeleteJob:
		d := msg.Data.(MgrMsgDeleteJob)
		c.removeJobContext(d.JobId)
		c.removePendingJob(removeJobMsg{jobId: d.JobId, force: d.Force, typ: MgrTypeDeleteJob})
		submit(func() {
			c.runner.DeleteTaskOfJob(d.JobId, d.Operation, d.Force)
		})
	case MgrTypeCancelTask:
		d := msg.Data.(MgrMsgCancelTask)
		submit(func() {
			c.runner.DeleteTask(d.TaskId, d.Operation, d.Force)
		})
	case MgrTypeDeleteTask:
		d := msg.Data.(MgrMsgDeleteTask)
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

func (c *centerImpl) removePendingJob(msg removeJobMsg) {
	c.pendingJobChDel <- msg
}

func (c *centerImpl) rolloutLoop(l []*PendingJobItem) {
	var pendingJobs []*PendingJobItem
	pendingJobs = append(pendingJobs, l...)
	delPendingAt := func(index int) {
		pendingJobs = append(pendingJobs[:index], pendingJobs[index+1:]...)
	}
	clearJob := func(v *PendingJobItem, index int) {
		// TODO More reasonable deletion strategy fro JobContext
		if v.Context.Status != StatusInProgress {
			c.removeJobContext(v.Context.JobId)
		}
		delPendingAt(index)
	}

	tick := time.NewTicker(c.opt.ScheduleInterval)
	// schedule loop
	for {
		select {
		case <-c.ctx.Done():
			return
		case p := <-c.pendingJobCh:
			pendingJobs = append(pendingJobs, &p)
		case del := <-c.pendingJobChDel:
			for i, j := range pendingJobs {
				if j.Context.JobId == del.jobId {
					if del.force {
						delPendingAt(i)
						c.removeJobContext(del.jobId)
					} else {
						// remove unscheduled task
						j.Tasks = ongoingTasks(j.Tasks)
						if len(j.Tasks) == 0 {
							delPendingAt(i)
							// TODO More reasonable deletion strategy fro JobContext
							if j.Context.Status != StatusInProgress {
								c.removeJobContext(del.jobId)
							}
						} else {

						}
					}
				}
			}
			continue
		case <-c.getPendingReqCh:
			if c.getPendingRespCh != nil {
				_ = c.pool.Submit(func() {
					defer func() {
						if err := recover(); err != nil { // getPendingRespCh maybe nil
							log.Errorf("JobCenter get pending job error: %v", err)
						}
					}()
					var cp []PendingJobItem
					for _, j := range pendingJobs {
						cp = append(cp, *j)
					}
					c.getPendingRespCh <- cp
				})
			}
		case njc := <-c.updateJobContextCh:
			for _, p := range pendingJobs {
				if njc.jobId == p.Context.JobId {
					if njc.TimeoutConfig != nil {
						p.Context.TimeoutConfig = njc.TimeoutConfig
					}
					if njc.RetryConfig != nil {
						p.Context.RetryConfig = njc.RetryConfig
					}
					c.setJobContext(p.Context.JobId, p.Context)
					break
				}
			}
		case <-tick.C:
		}
		l := len(pendingJobs)
		for i := 0; i < l; i++ {
			v := pendingJobs[i]

			if next, remove := c.jobScheduleFilter(v); !next {
				if remove {
					clearJob(v, i)
					l = len(pendingJobs)
				}
				continue
			}

			if next, remove := c.taskTimeoutFilter(v); !next {
				if remove {
					clearJob(v, i)
					l = len(pendingJobs)
				}
				continue
			}

			jc := &v.Context
			c.setJobContext(v.Context.JobId, *jc)

			// rollout count

			var rolloutTasks []Task
			rolloutCount := len(v.Tasks)
			if jc.RolloutConfig != nil && jc.RolloutConfig.MaxPerMinute > 0 {
				rolloutCount = jobNextRolloutCount(jc.RolloutConfig.MaxPerMinute, *v)
			}
			rolloutTasks = v.Tasks[:rolloutCount]
			v.Tasks = v.Tasks[rolloutCount:]

			// do rollout
			c.runner.PutTasks(jc.Operation, rolloutTasks)

			v.RolloutStat = append(v.RolloutStat, struct {
				Time  time.Time
				Count int
			}{Time: time.Now(), Count: len(rolloutTasks)})

			log.Infof("JobCenter scheduled jobId=%q, taskCount=%d, rolloutCount=%d",
				jc.JobId, len(v.Tasks), len(rolloutTasks))

			// delete from pending for rollout completed job
			// but keep it's JobContext for may be sum tasks of the job is running,
			// the JobContext will be removed after it's all tasks are done
			if len(v.Tasks) == 0 {
				delPendingAt(i)
				l = len(pendingJobs)
			}
		}
	}
}

func isJobScheduleInTime(jc *JobContext) bool {
	return !isJobScheduleBeforeStartTime(jc) && !isJobScheduleAfterEndTime(jc)
}
func isJobScheduleBeforeStartTime(jc *JobContext) bool {
	return jc.SchedulingConfig != nil && time.Now().Before(jc.SchedulingConfig.StartTime)
}
func isJobScheduleAfterEndTime(jc *JobContext) bool {
	return jc.SchedulingConfig != nil &&
		jc.SchedulingConfig.EndTime != nil &&
		time.Now().After(*jc.SchedulingConfig.EndTime)
}
func (c *centerImpl) jobScheduleFilter(j *PendingJobItem) (next bool, remove bool) {
	jc := &j.Context
	if isJobScheduleInTime(&j.Context) {
		next = true
		remove = false
		// update status
		if jc.Status == StatusWaiting {
			if err := c.repo.UpdateJob(
				c.ctx, jc.JobId,
				map[string]any{"status": StatusInProgress, "started_at": time.Now()},
			); err != nil {
				log.Errorf("JobCenter update job status, job=%q, status=%q, error: %v",
					jc.JobId, StatusInProgress, err)
			}
			jc.Status = StatusInProgress
		}

		if isJobToTerminal(jc.Status) {
			j.Tasks = ongoingTasks(j.Tasks)
		}
		if len(j.Tasks) == 0 {
			next = false
			remove = true
			_ = c.doFinishJob(jc.JobId, jc.Status)
		}
		return
	}

	// out of schedule time

	if isJobScheduleBeforeStartTime(&j.Context) {
		// for next time check
		return false, false
	}

	// after schedule end time

	// stop rollout, continue ongoing tasks
	if jc.SchedulingConfig.EndBehavior == ScheduleEndBehaviorStopRollout {
		next = true
		remove = false
		j.Tasks = ongoingTasks(j.Tasks)
		return
	}

	// cancel job
	jc.Status = StatusCanceled
	endBehavior := jc.SchedulingConfig.EndBehavior
	var force bool
	switch endBehavior {
	case ScheduleEndBehaviorCancel:
		next = true
		remove = false
		force = false
	case ScheduleEndBehaviorForceCancel:
		next = false
		remove = true
		force = true
	default:
		log.Fatalf("JobCenter schedule filter got unknown endBehavior=%q", endBehavior)
	}

	// notify runner cancel tasks
	c.runner.CancelTaskOfJob(jc.JobId, jc.Operation, force)

	if err := c.repo.CancelTasks(c.ctx, jc.JobId, force); err != nil {
		log.Errorf("JobCenter cancel tasks jobId=%q, error: %v", jc.JobId, err)
		next = false
		remove = false
	}
	if err := c.repo.UpdateJob(c.ctx, jc.JobId, map[string]any{
		"status":         StatusCanceled,
		"completed_at":   time.Now(),
		"force_canceled": force,
	}); err != nil {
		log.Errorf("JobCenter cancel job jobId=%q, error: %v", jc.JobId, err)
		next = false
		remove = false
	}
	if next {
		// get tasks ongoing for job after cancel
		if nl, err := c.repo.GetTasksOfJob(c.ctx, jc.JobId, []TaskStatus{TaskSent, TaskInProgress}); err != nil {
			log.Errorf("JobCenter get tasks of job jobId=%q, error: %v", jc.JobId, err)
			next = false
			remove = false
		} else {
			remove = false
			j.Tasks = toTasks(nl)
		}
	}
	return
}

func (c *centerImpl) preloadPendingJobs() (jobs []*PendingJobItem) {
	l, err := c.repo.GetPendingJobs(c.ctx)
	if err != nil {
		log.Fatalf("JobCenter get pending jobs: %v", err)
	}
	for _, j := range l {
		if len(j.Tasks) == 0 {
			log.Warnf("JobCenter job has no pending task, to terminate it, jobId=%q", j.JobId)
			_ = c.doFinishJob(j.JobId, j.Status)
			continue
		}
		if d, err := toDetail(j, []TaskStatusCount{}); err != nil {
			log.Fatalf("JobCenter convert entity to Detail, jobId=%q, error: %v", j.JobId, err)
		} else {
			p := PendingJobItem{
				Context: JobContext{
					JobId: d.JobId, Operation: d.Operation, JobDoc: d.JobDoc,
					SchedulingConfig: d.SchedulingConfig, RolloutConfig: d.RolloutConfig,
					RetryConfig: d.RetryConfig, TimeoutConfig: d.TimeoutConfig,
					Status: d.Status, StartedAt: d.StartedAt,
				},
				Tasks: toTasks(j.Tasks),
			}
			jobs = append(jobs, &p)
		}
	}
	return
}

func (c *centerImpl) taskTimeoutFilter(p *PendingJobItem) (next bool, remove bool) {
	next = true
	remove = false
	if p.Context.TimeoutConfig == nil {
		return
	}
	inProgressMinute := p.Context.TimeoutConfig.InProgressMinutes
	if inProgressMinute <= 0 {
		return
	}

	remainTasks := []Task{}
	for _, t := range p.Tasks {
		timeout := checkTimeoutTask(c.ctx, &t, inProgressMinute, c.repo)
		if !timeout {
			remainTasks = append(remainTasks, t)
		}
	}
	if len(remainTasks) == 0 {
		remove = true
		next = false
	}
	p.Tasks = remainTasks

	return
}

func checkTimeoutTask(ctx context.Context, t *Task, inProgressMinutes int, repo Repo) (timeout bool) {
	timeout = false
	if t.Status != TaskInProgress || t.StartedAt == nil {
		return
	}
	st := time.UnixMilli(*t.StartedAt)
	if time.Since(st).Seconds() < float64(inProgressMinutes*60) {
		return
	}
	_ = repo.UpdateTask(ctx, t.TaskId, map[string]any{
		"status":       TaskTimeOut,
		"completed_at": time.Now(),
	})
	return true
}

func jobNextRolloutCount(maxCountPerMinute int, p PendingJobItem) int {
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
				if c.checkFinishJob(k) {
					delete(pendingCheckJobs, k)
				}
			}
		case msg = <-tcCh:
			log.Infof("JobCenter watched task change, jobId=%q, taskId=%d, thingId=%q, "+
				"status=%q, progress=%d, statusDetails=%v",
				msg.Task.JobId, msg.Task.TaskId, msg.Task.ThingId,
				msg.Status, msg.Progress, msg.StatusDetails)
		}
		if isTaskTerminal(msg.Status) {
			pendingCheckJobs[msg.Task.JobId] = struct{}{}
		}

		// TODO: more for task change
	}
}

func (c *centerImpl) checkFinishJob(jobId string) (finished bool) {
	defer func() {
		if finished {
			c.removeJobContext(jobId)
			c.removePendingJob(removeJobMsg{jobId: jobId, force: true})
		}
	}()
	res, err := c.repo.CountTaskStatus(c.ctx, jobId)
	if err != nil {
		log.Errorf("JobCenter get task status count error: %v", err)
		return false
	}
	for _, taskStatusCount := range res {
		if !isTaskTerminal(taskStatusCount.Status) {
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
		return true
	}
	_ = c.doFinishJob(jobId, j.Status)
	return true
}

func (c *centerImpl) doFinishJob(jobId string, status Status) error {
	var st Status
	switch status {
	case StatusCanceling:
		st = StatusCanceled
	case StatusRemoving:
		if _, err := c.repo.DeleteJob(c.ctx, jobId, true); err != nil {
			return err
		} else {
			log.Errorf("JobCenter delete job, jobId=%q, error: %v", jobId, err)
			return nil
		}
	case StatusWaiting, StatusInProgress:
		st = StatusCompleted
	case StatusCanceled:
		// do nothing
		// job which canceled without force may have ongoing tasks
		// so that the job may be checked for finish more than one time
	default:
		log.Warnf("JobCenter unexpected job status when check, jobId=%q, status=%q", jobId, status)
		return nil
	}
	err := c.repo.UpdateJob(c.ctx, jobId, map[string]any{
		"status":       st,
		"completed_at": time.Now(),
	})
	if err != nil {
		log.Errorf("JobCenter update job finish, jobId=%q, %v", jobId, err)
		return err
	} else {
		log.Infof("JobCenter job finished, jobId=%q, status=%q", jobId, st)
		return nil
	}
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

// ------------------------- helper func -------------------------

func isJobToTerminal(s Status) bool {
	if s == StatusCanceling || s == StatusCanceled || s == StatusRemoving || s == StatusCompleted {
		return true
	}
	return false
}

func isJobTerminal(s Status) bool {
	if s == StatusCanceled || s == StatusCompleted {
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

func ongoingTasks(l []Task) (ongoing []Task) {
	for _, t := range l {
		if isTaskOngoing(t.Status) {
			ongoing = append(ongoing, t)
		}
	}
	return
}
