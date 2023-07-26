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
				if re := r.doInvokeDirectMethod(*t, req); re.Err != nil {
					log.Errorf("JobRunner do invoke direct method, jobId=%q taskId=%d thingId=%s : %v",
						jc.JobId, t.TaskId, t.ThingId, re.Err)
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
				r.innerTaskChangeCh <- TaskChangeMsg{Task: *t, Status: TaskSent}
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
