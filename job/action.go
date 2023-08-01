package job

import (
	"fmt"
	"time"

	"ruff.io/tio/shadow"
)

func (r *runnerImpl) doInvokeDirectMethod(t Task, req InvokeDirectMethodReq) TaskChangeMsg {
	resp, err := r.methodHandler.InvokeMethod(r.ctx, shadow.MethodReqMsg{
		ThingId:     t.ThingId,
		Method:      req.Method,
		RespTimeout: req.RespTimeout,
		Req: shadow.MethodReq{
			ClientToken: fmt.Sprintf("job-%s-%d", t.ThingId, time.Now().Nanosecond()),
			Data:        req.Data,
		},
	})

	tcMsg := TaskChangeMsg{Task: t}

	if err != nil {
		tcMsg.Status = TaskFailed
		tcMsg.StatusDetails = StatusDetails{
			"code":    500,
			"message": err.Error(),
		}
	} else {
		tcMsg.StatusDetails = StatusDetails{
			"code":    resp.Code,
			"message": resp.Message,
			"data":    resp.Data,
		}
		if resp.Code != 200 && resp.Code != 0 {
			tcMsg.Status = TaskFailed
		} else {
			tcMsg.Status = TaskSucceeded
		}
	}

	return tcMsg
}

func (r *runnerImpl) doUpdateShadow(t Task, req UpdateShadowReq) TaskChangeMsg {
	_, err := r.shadowSetter.SetDesired(r.ctx, t.ThingId, shadow.StateReq{
		ClientToken: fmt.Sprintf("job-%d-%d", t.TaskId, time.Now().UnixNano()),
		State:       shadow.StateDR{Desired: req.State.Desired},
	})

	if err != nil {
		return TaskChangeMsg{
			Task: t,
			StatusDetails: StatusDetails{
				"code":    500,
				"message": err.Error(),
			},
			Status: TaskFailed,
		}
	} else {
		return TaskChangeMsg{
			Task:   t,
			Status: TaskSucceeded,
		}
	}
}
