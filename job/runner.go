package job

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
	"time"
)

func NewRunner(
	repo Repo,
	pubSub connector.PubSub, conn connector.ConnectChecker,
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

		// for actions

		pubSub:        pubSub,
		conn:          conn,
		methodHandler: methodHandler,
		shadowSetter:  shadowSetter,

		// channels for task change
		innerTaskChangeCh: make(chan TaskChangeMsg),
		outTaskChangeCh:   make(chan TaskChangeMsg),

		// channels for direct method task
		sysOpTaskCh:    make(chan []Task),
		sysOpTaskDelCh: make(chan []int64),

		// channels for get tasks
		getPendingTasksOfCustomReqCh:  make(chan struct{}),
		getPendingTasksOfCustomRespCh: nil,
		getPendingTasksOfSysReqCh:     make(chan struct{}),
		getPendingTasksOfSysRespCh:    nil,
	}
}

type runnerImpl struct {
	ctx context.Context

	repo Repo

	jcGetter ctxGetter
	pool     *ants.Pool

	pubSub        connector.PubSub
	conn          connector.ConnectChecker
	methodHandler shadow.MethodHandler
	shadowSetter  shadow.StateDesiredSetter

	innerTaskChangeCh chan TaskChangeMsg
	outTaskChangeCh   chan TaskChangeMsg

	sysOpTaskCh    chan []Task
	sysOpTaskDelCh chan []int64

	// channels for get tasks
	getPendingTasksOfCustomReqCh  chan struct{}
	getPendingTasksOfCustomRespCh chan []Task
	getPendingTasksOfSysReqCh     chan struct{}
	getPendingTasksOfSysRespCh    chan []Task

	thingTaskQueues map[string]TaskQueue // thingId->[]Task, for general task

}

var _ Runner = &runnerImpl{}

func (r *runnerImpl) Start(ctx context.Context, jcGetter ctxGetter) {
	r.ctx = ctx
	r.jcGetter = jcGetter
	go r.watchTaskChangeLoop()
	go r.sysOpTaskLoop(r.sysOpTaskCh, r.sysOpTaskDelCh)
}

func (r *runnerImpl) OnTaskChange() <-chan TaskChangeMsg {
	return r.outTaskChangeCh
}

func (r *runnerImpl) PutTasks(operation string, l []Task) {
	switch operation {
	case SysOpDirectMethod, SysOpUpdateShadow:
		r.sysOpTaskCh <- l
	default:
		// TODO custom
	}
}

func (r *runnerImpl) GetPendingTasksOfSys(op string) []Task {
	switch op {
	case SysOpDirectMethod, SysOpUpdateShadow:
		r.getPendingTasksOfSysRespCh = make(chan []Task, 1)
		defer func() {
			r.getPendingTasksOfSysRespCh = nil
		}()
		r.getPendingTasksOfSysReqCh <- struct{}{}
		return <-r.getPendingTasksOfSysRespCh

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
	case SysOpDirectMethod, SysOpUpdateShadow:
		var taskIds []int64
		for _, t := range tl {
			taskIds = append(taskIds, t.TaskId)
		}
		r.sysOpTaskDelCh <- taskIds
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
	case SysOpDirectMethod, SysOpUpdateShadow:
		var taskIds []int64
		for _, t := range tl {
			taskIds = append(taskIds, t.TaskId)
		}
		r.sysOpTaskDelCh <- taskIds
		log.Debugf("JobRunner sent msg for delete tasks of direct method: %v", taskIds)
	default:
	}
}

func (r *runnerImpl) DeleteTask(taskId int64, operation string, force bool) {
	switch operation {
	case SysOpDirectMethod, SysOpUpdateShadow:
		r.sysOpTaskDelCh <- []int64{taskId}
	default:
	}
}

func (r *runnerImpl) CancelTask(taskId int64, operation string, force bool) {
	switch operation {
	case SysOpDirectMethod, SysOpUpdateShadow:
		r.sysOpTaskDelCh <- []int64{taskId}
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
				if chMsg.Task.Operation != SysOpDirectMethod &&
					chMsg.Task.Operation != SysOpUpdateShadow {
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
			msg.StatusDetails, msg.Task.JobId, msg.Task.TaskId, err)
	}
	err = r.repo.ExecWithTx(func(txRepo Repo) error {
		t, err := txRepo.GetTask(r.ctx, msg.Task.TaskId)
		if err != nil {
			return err
		}
		if t == nil {
			return errors.New("task not found")
		}
		if isTaskTerminal(t.Status) {
			return fmt.Errorf("task is terminal at status=%q", t.Status)
		}
		err = txRepo.UpdateTask(r.ctx, msg.Task.TaskId, map[string]any{
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
			msg.Task.JobId, msg.Task.TaskId, msg.Status, err)
	}
	log.Debugf("JobRunner update task status, jobId=%q, taskId=%d, status=%q, progress=%v",
		msg.Task.JobId, msg.Task.TaskId, msg.Status, msg.Progress)
}

func (r *runnerImpl) sysOpTaskLoop(addCh <-chan []Task, delCh <-chan []int64) {
	defer func() {
		log.Info("JobRunner system operation loop method loop exit")
	}()
	concurrentOnTick := 10
	// The task queue is only used in this go routine for lock-free
	curQ := NewTaskQueue()
	offlineThingTasks := map[string][]Task{}

	tick := time.NewTicker(time.Millisecond * 50)
	onConn := r.conn.OnConnect()
	for {
		select {
		case <-r.ctx.Done():
			log.Debugf("JobRunner system operation exit cause context closed")
			return
		case tl := <-addCh:
			for _, t := range tl {
				st := t
				log.Debugf("JobRunner push task %d", st.TaskId)
				curQ.Push(&st)
			}
			continue
		case dl := <-delCh:
			for _, id := range dl {
				_ = curQ.RemoveById(id)
			}
			continue
		case e := <-onConn:
			if e.EventType == connector.EventConnected {
				log.Debugf("JobRunner got thing online thingId=%q", e.ThingId)
				if l, ok := offlineThingTasks[e.ThingId]; ok {
					delete(offlineThingTasks, e.ThingId)
					for _, t := range l {
						curQ.Push(&t)
					}
					log.Debugf("JobRunner got thing online thingId=%q, taskCount=%d put back tasks done",
						e.ThingId, len(l))
				}
			}
		case <-r.getPendingTasksOfSysReqCh:
			_ = r.pool.Submit(func() {
				l := curQ.GetTasks()
				for _, v := range offlineThingTasks {
					l = append(l, v...)
				}
				r.getPendingTasksOfSysRespCh <- l
			})
		case <-tick.C:
			// do task below
		}

		c := 0
		for curQ.Size() > 0 && c < concurrentOnTick {
			c++
			t := curQ.Pop()
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

			var submitErr error = nil
			if t.Operation == SysOpDirectMethod {
				// check thing connection online
				if online, err := r.conn.IsConnected(t.ThingId); err != nil {
					log.Errorf("JobRunner check thing online, thingId=%q, error: %v", t.ThingId, err)
				} else if !online {
					if l, ok := offlineThingTasks[t.ThingId]; ok {
						offlineThingTasks[t.ThingId] = append(l, *t)
					} else {
						offlineThingTasks[t.ThingId] = []Task{*t}
					}
					log.Debugf("JobRunner put task to offline map taskId=%d", t.TaskId)
					continue
				}
				submitErr = r.submitDirectMethodTaskToPool(jc, t)
			} else if t.Operation == SysOpUpdateShadow {
				submitErr = r.submitUpdateShadowTaskToPool(jc, t)
			}

			if submitErr != nil {
				log.Warnf("JobRunner submit task error, jobId=%q, taskId=%d, thingId=%q, error: %v",
					t.JobId, t.TaskId, t.ThingId, submitErr)
				curQ.Push(t)

				// maybe pool is full, break for next tick
				break
			} else {
				log.Infof("JobRunner submit task success, jobId=%q, taskId=%d, thingId=%q",
					jc.JobId, t.TaskId, t.ThingId)
				if t.Status == TaskQueued {
					r.innerTaskChangeCh <- TaskChangeMsg{Task: *t, Status: TaskSent}
				}
			}
		}
	}
}

func (r *runnerImpl) submitDirectMethodTaskToPool(jc *JobContext, t *Task) error {
	return r.pool.Submit(func() {
		var req InvokeDirectMethodReq
		if jc.JobDoc == nil {
			log.Errorf("JobRunner unexpected job doc is nil for invoke direct method! jobId=%q, jobDoc=%v",
				jc.JobId, jc.JobDoc)
		}

		jBuf, err := json.Marshal(jc.JobDoc)
		if err != nil {
			log.Errorf("JobRunner unexpected job doc is nil for invoke direct method! jobId=%q, jobDoc=%v",
				jc.JobId, jc.JobDoc)
		}
		if err := json.Unmarshal(jBuf, &req); err != nil {
			// job doc should be checked before job created
			log.Errorf("JobRunner unexpected job doc for invoke direct method! jobId=%q, jobDoc=%v",
				jc.JobId, jc.JobDoc)
		}

		re := r.doInvokeDirectMethod(*t, req)
		if re.Err != nil {
			log.Errorf("JobRunner do invoke direct method, jobId=%q taskId=%d thingId=%s : %v",
				jc.JobId, t.TaskId, t.ThingId, re.Err)
		}
		// notify result
		r.innerTaskChangeCh <- re
	})
}

func (r *runnerImpl) submitUpdateShadowTaskToPool(jc *JobContext, t *Task) error {
	return r.pool.Submit(func() {
		var req UpdateShadowReq
		if jc.JobDoc == nil {
			log.Errorf("JobRunner unexpected job doc is nil for update shadow! jobId=%q, jobDoc=%v",
				jc.JobId, jc.JobDoc)
		}

		jBuf, err := json.Marshal(jc.JobDoc)
		if err != nil {
			log.Errorf("JobRunner unexpected job doc is nil for update shadow! jobId=%q, jobDoc=%v",
				jc.JobId, jc.JobDoc)
		}
		if err := json.Unmarshal(jBuf, &req); err != nil {
			// job doc should be checked before job created
			log.Errorf("JobRunner unexpected job doc for update shadow! jobId=%q, jobDoc=%v",
				jc.JobId, jc.JobDoc)
		}

		re := r.doUpdateShadow(*t, req)
		if re.Err != nil {
			log.Errorf("JobRunner do update shadow, jobId=%q taskId=%d thingId=%s : %v",
				jc.JobId, t.TaskId, t.ThingId, re.Err)
		}
		// notify result
		r.innerTaskChangeCh <- re
	})
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
