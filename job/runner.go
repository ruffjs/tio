package job

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"log"

	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	"ruff.io/tio/shadow"
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
		sysOpTaskDelCh: make(chan deleteTaskMsg),

		// channels for get tasks
		getPendingTasksOfCustomReqCh:  make(chan struct{}),
		getPendingTasksOfCustomRespCh: nil,
		getPendingTasksOfSysReqCh:     make(chan struct{}),
		getPendingTasksOfSysRespCh:    nil,
	}
}

type deleteTaskMsg struct {
	jobId string
	tasks []int64
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
	sysOpTaskDelCh chan deleteTaskMsg
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
	if IsSysOp(operation) {
		r.sysOpTaskCh <- l
	} else {
		// TODO custom
	}
}

func (r *runnerImpl) GetPendingTasksOfSys(op string) []Task {
	if IsSysOp(op) {
		r.getPendingTasksOfSysRespCh = make(chan []Task, 1)
		defer func() {
			r.getPendingTasksOfSysRespCh = nil
		}()
		r.getPendingTasksOfSysReqCh <- struct{}{}
		return <-r.getPendingTasksOfSysRespCh
	}
	return []Task{}
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
	if IsSysOp(operation) {
		r.sysOpTaskDelCh <- deleteTaskMsg{jobId: jobId}
	} else {
		// TODO: custom
	}
}

func (r *runnerImpl) CancelTaskOfJob(jobId, operation string, force bool) {
	if IsSysOp(operation) {
		r.sysOpTaskDelCh <- deleteTaskMsg{jobId: jobId}
		slog.Debug("JobRunner sent msg for delete tasks of system operation", "jobId", jobId)
	} else {
		// TODO: custom
	}
}

func (r *runnerImpl) DeleteTask(taskId int64, operation string, force bool) {
	if IsSysOp(operation) {
		r.sysOpTaskDelCh <- deleteTaskMsg{tasks: []int64{taskId}}
	} else {
		// TODO: custom
	}
}

func (r *runnerImpl) CancelTask(taskId int64, operation string, force bool) {
	if IsSysOp(operation) {
		r.sysOpTaskDelCh <- deleteTaskMsg{tasks: []int64{taskId}}
	} else {
		// TODO: custom
	}
}

func (r *runnerImpl) watchTaskChangeLoop() {
	for {
		select {
		case <-r.ctx.Done():
			slog.Debug("JobRunner task change watcher exit cause context closed")
			return
		case chMsg := <-r.innerTaskChangeCh:
			r.updateTaskStatus(chMsg)
			r.outTaskChangeCh <- chMsg

			if isTaskTerminal(chMsg.Status) {
				if IsCustomOp(chMsg.Task.Operation) {
					// TODO: update thing task queue
				}
			}
		}
	}
}

func (r *runnerImpl) updateTaskStatus(msg TaskChangeMsg) {
	sdBuf, err := json.Marshal(msg.StatusDetails)
	if err != nil {
		slog.Error("JobRunner update task status, unexpected marshal statusDetails",
			"statusDetails", msg.StatusDetails, "jobId", msg.Task.JobId, "taskId", msg.Task.TaskId, "error", err)
	}
	err = r.repo.ExecWithTx(func(txRepo Repo) error {
		t, er := txRepo.GetTask(r.ctx, msg.Task.TaskId)
		if er != nil {
			return err
		}
		if t == nil {
			return errors.New("task not found")
		}
		if isTaskTerminal(t.Status) {
			return fmt.Errorf("task is terminal at status=%q", t.Status)
		}
		toUpdate := map[string]any{
			"status":         msg.Status,
			"progress":       msg.Progress,
			"status_details": sdBuf,
		}
		if isTaskTerminal(t.Status) {
			toUpdate["completed_at"] = time.Now()
		}
		er = txRepo.UpdateTask(r.ctx, msg.Task.TaskId, toUpdate)
		if er != nil {
			return err
		}
		return nil
	})
	if err != nil {
		slog.Error("JobRunner update task status", "jobId", msg.Task.JobId, "taskId", msg.Task.TaskId, "status", msg.Status, "error", err)
	}
	slog.Debug("JobRunner update task status", "jobId", msg.Task.JobId, "taskId", msg.Task.TaskId, "status", msg.Status, "progress", msg.Progress)
}

func (r *runnerImpl) sysOpTaskLoop(addCh <-chan []Task, delCh <-chan deleteTaskMsg) {
	defer func() {
		slog.Info("JobRunner system operation loop method loop exit")
	}()
	concurrentOnTick := 10
	// The task queue is only used in this go routine for lock-free
	curQ := NewTaskQueue()
	offlineThingTasks := map[string][]Task{}

	tick := time.NewTicker(time.Millisecond * 50)
	retryTick := time.NewTicker(2 * time.Second)
	defer retryTick.Stop()
	for {
		select {
		case <-r.ctx.Done():
			slog.Debug("JobRunner system operation exit cause context closed")
			return
		case tl := <-addCh:
			for _, t := range tl {
				st := t
				slog.Debug("JobRunner push task", "taskId", st.TaskId)
				curQ.Push(&st)
			}
			continue
		case dl := <-delCh:
			if len(dl.tasks) > 0 {
				for _, id := range dl.tasks {
					_ = curQ.RemoveById(id)
				}
				for k, v := range offlineThingTasks {
					var vn []Task
					for _, t := range v {
						for _, id := range dl.tasks {
							if t.TaskId == id {
								break
							}
						}
						vn = append(vn, t)
					}
					offlineThingTasks[k] = vn
				}
			} else if dl.jobId != "" {
				l := curQ.GetTasks()
				for _, t := range l {
					if t.JobId == dl.jobId {
						curQ.RemoveById(t.TaskId)
					}
				}
				for k, v := range offlineThingTasks {
					var vn []Task
					for _, t := range v {
						if t.JobId != dl.jobId {
							vn = append(vn, t)
						}
					}
					offlineThingTasks[k] = vn
				}
			}
			continue
		case <-retryTick.C:
			for thingId, l := range offlineThingTasks {
				if online, err := r.conn.IsConnected(thingId); err == nil && online {
					delete(offlineThingTasks, thingId)
					for _, t := range l {
						curQ.Push(&t)
					}
					slog.Debug("JobRunner retry: thing online, put back tasks", "thingId", thingId, "taskCount", len(l))
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
				slog.Warn("JobRunner job context is nil, maybe deleted", "jobId", t.JobId)
				continue
			}
			if isJobToTerminal(jc.Status) {
				slog.Info("JobRunner job is going to terminal status", "status", jc.Status, "taskId", t.TaskId, "thingId", t.ThingId)
				continue
			}

			var submitErr error = nil
			if IsDirectMethodOp(t.Operation) {
				// check thing connection online
				if online, err := r.conn.IsConnected(t.ThingId); err != nil {
					slog.Error("JobRunner check thing online", "thingId", t.ThingId, "error", err)
				} else if !online {
					if l, ok := offlineThingTasks[t.ThingId]; ok {
						offlineThingTasks[t.ThingId] = append(l, *t)
					} else {
						offlineThingTasks[t.ThingId] = []Task{*t}
					}
					slog.Debug("JobRunner put task to offline map", "taskId", t.TaskId)
					continue
				}
				submitErr = r.submitDirectMethodTaskToPool(jc, t)
			} else if IsUpdateShadowOp(t.Operation) {
				submitErr = r.submitUpdateShadowTaskToPool(jc, t)
			}

			if submitErr != nil {
				slog.Warn("JobRunner submit task error", "jobId", t.JobId, "taskId", t.TaskId, "thingId", t.ThingId, "error", submitErr)
				curQ.Push(t)

				// maybe pool is full, break for next tick
				break
			} else {
				slog.Info("JobRunner submit task success", "jobId", jc.JobId, "taskId", t.TaskId, "thingId", t.ThingId)
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
			slog.Error("JobRunner unexpected job doc is nil for invoke direct method", "jobId", jc.JobId, "jobDoc", jc.JobDoc)
		}

		jBuf, err := json.Marshal(jc.JobDoc)
		if err != nil {
			slog.Error("JobRunner unexpected job doc is nil for invoke direct method", "jobId", jc.JobId, "jobDoc", jc.JobDoc)
		}
		if err := json.Unmarshal(jBuf, &req); err != nil {
			// job doc should be checked before job created
			slog.Error("JobRunner unexpected job doc for invoke direct method", "jobId", jc.JobId, "jobDoc", jc.JobDoc)
		}

		re := r.doInvokeDirectMethod(*t, req)
		if re.Err != nil {
			slog.Error("JobRunner do invoke direct method", "jobId", jc.JobId, "taskId", t.TaskId, "thingId", t.ThingId, "error", re.Err)
		}
		// notify result
		r.innerTaskChangeCh <- re
	})
}

func (r *runnerImpl) submitUpdateShadowTaskToPool(jc *JobContext, t *Task) error {
	return r.pool.Submit(func() {
		var req UpdateShadowReq
		if jc.JobDoc == nil {
			slog.Error("JobRunner unexpected job doc is nil for update shadow", "jobId", jc.JobId, "jobDoc", jc.JobDoc)
		}

		jBuf, err := json.Marshal(jc.JobDoc)
		if err != nil {
			slog.Error("JobRunner unexpected job doc is nil for update shadow", "jobId", jc.JobId, "jobDoc", jc.JobDoc)
		}
		if err := json.Unmarshal(jBuf, &req); err != nil {
			// job doc should be checked before job created
			slog.Error("JobRunner unexpected job doc for update shadow", "jobId", jc.JobId, "jobDoc", jc.JobDoc)
		}

		re := r.doUpdateShadow(*t, req)
		if re.Err != nil {
			slog.Error("JobRunner do update shadow", "jobId", jc.JobId, "taskId", t.TaskId, "thingId", t.ThingId, "error", re.Err)
		}
		// notify result
		r.innerTaskChangeCh <- re
	})
}
