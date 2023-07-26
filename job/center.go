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
	var pendingJobs []*PendingJobItem
	delPendingAt := func(index int) {
		pendingJobs = append(pendingJobs[:index], pendingJobs[index+1:]...)
	}

	// preload pending jobs from db

	l, err := c.repo.GetPendingJobs(c.ctx)
	if err != nil {
		log.Fatalf("JobCenter get pending jobs: %v", err)
	}
	for _, j := range l {
		if len(j.Tasks) == 0 {
			log.Warnf("JobCenter job has no pending task, to terminate it, jobId=%q", j.JobId)
			// TODO finish job by current job status
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
			pendingJobs = append(pendingJobs, &p)
		}
	}

	tick := time.NewTicker(c.opt.ScheduleInterval)
	// schedule loop
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
		case <-tick.C:
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

				v.RolloutStat = append(v.RolloutStat, struct {
					Time  time.Time
					Count int
				}{Time: time.Now(), Count: len(rolloutTasks)})

				// do rollout
				c.runner.PutTasks(jc.Operation, rolloutTasks)

				log.Infof("JobCenter scheduled jobId=%q, taskCount=%d, rolloutCount=%d",
					jc.JobId, len(tasks), len(rolloutTasks))

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
				msg.Task.JobId, msg.Task.TaskId, msg.Task.ThingId,
				msg.Status, msg.Progress, msg.StatusDetails)
		}
		if isTaskTerminal(msg.Status) {
			pendingCheckJobs[msg.Task.JobId] = struct{}{}
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
