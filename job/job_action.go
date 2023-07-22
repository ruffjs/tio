package job

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

func (r *runnerImpl) doInvokeDirectMethod(jobId string, taskId int64, thingId string, req InvokeDirectMethodReq) (TaskChangeMsg, error) {
	resp, er := r.methodHandler.InvokeMethod(r.ctx, shadow.MethodReqMsg{
		ThingId:     thingId,
		Method:      req.Method,
		RespTimeout: req.RespTimeout,
		Req: shadow.MethodReq{
			ClientToken: fmt.Sprintf("job-%s-%d", thingId, time.Now().Nanosecond()),
			Data:        req.Data,
		},
	})
	if er != nil && errors.Is(er, model.ErrDirectMethodThingOffline) {
		return TaskChangeMsg{}, er
	}

	tcMgr := TaskChangeMsg{
		JobId:     jobId,
		TaskId:    taskId,
		ThingId:   thingId,
		Operation: SysOpDirectMethod,
	}

	if er != nil {
		sd := StatusDetails{
			"code":    500,
			"message": er.Error(),
		}
		tcMgr.Status = TaskFailed
		tcMgr.StatusDetails = sd
	} else {
		sd := StatusDetails{
			"code":    resp.Code,
			"message": resp.Message,
			"data":    resp.Data,
		}
		tcMgr.StatusDetails = sd
		if resp.Code != 200 && resp.Code != 0 {
			tcMgr.Status = TaskFailed
		} else {
			tcMgr.Status = TaskSucceeded
		}
	}

	return tcMgr, nil
}
