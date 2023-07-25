package job

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"ruff.io/tio"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/pkg/uuid"
)

var idProvider = uuid.New()

type PageQuery struct {
	Status    Status `json:"status"`
	Operation string `json:"operation"`
	model.PageQuery
}

type Page model.PageData[Summary]

type TaskPageQuery struct {
	Status    TaskStatus `json:"status"`
	Operation string     `json:"operation"`
	model.PageQuery
}
type TaskPage model.PageData[TaskSummary]

type TaskStatusCount struct {
	Status TaskStatus
	Count  int
}

// MgrService Management Service
type MgrService interface {
	// Job API

	CreateJob(ctx context.Context, r CreateReq) (Detail, error)

	// UpdateJob Updated values for timeoutConfig take effect for only newly in-progress tasks.
	// Currently, in-progress tasks continue to launch with the previous timeout configuration.
	UpdateJob(ctx context.Context, jobId string, r UpdateReq) error

	// CancelJob  If  force is true, tasks with status "IN_PROGRESS" and "QUEUED" are canceled,
	// otherwise only tasks with status "QUEUED" are canceled.
	CancelJob(ctx context.Context, jobId string, r CancelReq, force bool) error

	// DeleteJob Deleting a job can take time, depending on the number of job executions created for the job and various other factors.
	// While the job is being deleted, the status of the job is shown as "REMOVING".
	// Attempting to delete or cancel a job whose status is already "REMOVING" results in an error.
	//
	// When force is true, you can delete a job which is "IN_PROGRESS".
	// Otherwise, you can only delete a job which is in a terminal state ("COMPLETED" or "CANCELED")
	DeleteJob(ctx context.Context, jobId string, force bool) (*Entity, error)

	GetJob(ctx context.Context, jobId string) (*Detail, error)
	QueryJob(ctx context.Context, q PageQuery) (Page, error)

	// Task API

	// CancelTask Task with QUEUED status can be canceled,
	// when force is true, task with QUEUED or IN_PROGRESS status can be canceled
	CancelTask(ctx context.Context, thingId, jobId string, r CancelTaskReq, force bool) error

	// DeleteTask When force is true, you can delete a task which is "IN_PROGRESS".
	// Otherwise, you can only delete a task which is in a terminal state ("SUCCEEDED", "FAILED", "REJECTED", "REMOVED" or "CANCELED")
	DeleteTask(ctx context.Context, thingId, jobId string, taskId int64, force bool) (*TaskEntity, error)

	GetTask(ctx context.Context, thingId, jobId string, taskId int64) (*Task, error)
	QueryTaskForThing(ctx context.Context, thingId string, q TaskPageQuery) (TaskPage, error)
	QueryTaskForJob(ctx context.Context, jobId string, q TaskPageQuery) (TaskPage, error)
}

type Repo interface {
	ExecWithTx(func(txRepo Repo) error) error

	// Job API

	CreateJob(ctx context.Context, j Entity) (Entity, error)
	UpdateJob(ctx context.Context, jobId string, m map[string]any) error

	// DeleteJob Tasks under the job will be deleted together
	DeleteJob(ctx context.Context, jobId string, force bool) (*Entity, error)
	GetJob(ctx context.Context, jobId string) (*Entity, error)
	QueryJob(ctx context.Context, q PageQuery) (model.PageData[Entity], error)

	GetPendingJobs(ctx context.Context) ([]Entity, error)

	// Task API

	CreateTasks(ctx context.Context, l []TaskEntity) ([]TaskEntity, error)
	UpdateTask(ctx context.Context, taskId int64, m map[string]any) error
	CancelTasks(ctx context.Context, jobId string, force bool) error
	DeleteTask(ctx context.Context, taskId int64) error
	GetTask(ctx context.Context, taskId int64) (*TaskEntity, error)
	QueryTask(ctx context.Context, jobId, thingId string, q TaskPageQuery) (model.PageData[TaskEntity], error)

	CountTaskStatus(ctx context.Context, jobId string) ([]TaskStatusCount, error)
	GetTasksOfJob(ctx context.Context, jobId string, status []TaskStatus) ([]TaskEntity, error)
}

func NewMgrService(repo Repo, idProvider tio.IdProvider, jc Center) MgrService {
	return mgrSvcImpl{repo, idProvider, jc}
}

var _ MgrService = &mgrSvcImpl{}

type mgrSvcImpl struct {
	repo       Repo
	idProvider tio.IdProvider
	jobCenter  Center
}

func (s mgrSvcImpl) CreateJob(ctx context.Context, p CreateReq) (Detail, error) {
	if err := p.valid(); err != nil {
		return Detail{}, err
	}
	e, err := toEntity(p)
	if err != nil {
		return Detail{}, err
	}
	if e.JobId == "" {
		e.JobId, err = idProvider.ID()
		if err != nil {
			return Detail{}, errors.WithMessage(model.ErrInternal, "generate jobId:"+err.Error())
		}
	}
	res, err := s.repo.CreateJob(ctx, e)
	if err != nil {
		return Detail{}, err
	}
	if d, err := toDetail(res, []TaskStatusCount{}); err != nil {
		return Detail{}, errors.WithMessage(model.ErrInternal, "to job detail")
	} else {
		s.jobCenter.ReceiveMgrMsg(MgrMsg{
			Typ: MgrTypeCreateJob,
			Data: MgrMsgCreateJob{
				TargetConfig: d.TargetConfig,
				JobContext: JobContext{
					JobId: d.JobId, Operation: d.Operation, JobDoc: d.JobDoc,
					SchedulingConfig: d.SchedulingConfig, RolloutConfig: d.RolloutConfig,
					RetryConfig: d.RetryConfig, TimeoutConfig: d.TimeoutConfig,
					Status: d.Status, StartedAt: d.StartedAt,
				},
			},
		})
		return d, nil
	}
}

// UpdateJob Updated values for timeoutConfig take effect for only newly in-progress tasks.
// Currently, in-progress tasks continue to launch with the previous timeout configuration.
func (s mgrSvcImpl) UpdateJob(ctx context.Context, jobId string, r UpdateReq) error {
	if err := r.valid(); err != nil {
		return err
	}
	toUpdate := map[string]any{}
	if r.Description != nil {
		toUpdate["description"] = *r.Description
	}
	if r.TimeoutConfig != nil {
		if buf, err := json.Marshal(*r.TimeoutConfig); err == nil {
			toUpdate["timeout_config"] = buf
		} else {
			return errors.WithMessage(model.ErrInvalidParams, "timeoutConfig: "+err.Error())
		}
	}
	if r.RetryConfig != nil {
		if buf, err := json.Marshal(*r.RetryConfig); err == nil {
			toUpdate["retry_config"] = buf
		} else {
			return errors.WithMessage(model.ErrInvalidParams, "retryConfig: "+err.Error())
		}
	}

	if len(toUpdate) == 0 {
		return nil
	}
	if err := s.repo.UpdateJob(ctx, jobId, toUpdate); err != nil {
		return errors.WithMessage(err, "update job")
	}
	s.jobCenter.ReceiveMgrMsg(MgrMsg{
		Typ:  MgrTypeUpdateJob,
		Data: MgrMsgUpdateJob{JobId: jobId, TimeoutConfig: *r.TimeoutConfig, RetryConfig: *r.RetryConfig},
	})
	return nil
}

func (s mgrSvcImpl) CancelJob(ctx context.Context, jobId string, r CancelReq, force bool) error {
	if err := r.valid(); err != nil {
		return err
	}

	toUpdate := map[string]any{
		"status":         StatusCanceling,
		"force_canceled": force,
	}
	if r.Comment != nil {
		toUpdate["comment"] = *r.Comment
	}
	if r.ReasonCode != nil {
		toUpdate["reason_code"] = r.ReasonCode
	}

	var job *Entity
	err := s.repo.ExecWithTx(func(txRepo Repo) error {
		j, err := txRepo.GetJob(ctx, jobId)
		job = j
		if err != nil {
			return err
		}
		if j == nil {
			return errors.WithMessage(model.ErrNotFound, "job")
		}
		if !force && j.Status == StatusInProgress {
			return errors.WithMessage(model.ErrInvalidParams, "can't cancel job which is in progress")
		}
		if err := txRepo.UpdateJob(ctx, jobId, toUpdate); err != nil {
			return err
		}
		if err := txRepo.CancelTasks(ctx, jobId, force); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "update db when cancel job")
	}
	s.jobCenter.ReceiveMgrMsg(MgrMsg{
		Typ:  MgrTypeCancelJob,
		Data: MgrMsgCancelJob{JobId: jobId, Force: force, Operation: job.Operation},
	})

	return nil
}

func (s mgrSvcImpl) DeleteJob(ctx context.Context, jobId string, force bool) (*Entity, error) {
	if j, err := s.repo.DeleteJob(ctx, jobId, force); err != nil {
		return nil, errors.WithMessagef(err, "delete job")
	} else {
		s.jobCenter.ReceiveMgrMsg(MgrMsg{
			Typ:  MgrTypeDeleteJob,
			Data: MgrMsgDeleteJob{JobId: jobId, Force: force, Operation: j.Operation},
		})
		return j, nil
	}
}

func (s mgrSvcImpl) GetJob(ctx context.Context, jobId string) (*Detail, error) {
	e, err := s.repo.GetJob(ctx, jobId)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	tsCount, err := s.repo.CountTaskStatus(ctx, jobId)
	if err != nil {
		return nil, err
	}
	d, err := toDetail(*e, tsCount)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s mgrSvcImpl) QueryJob(ctx context.Context, q PageQuery) (Page, error) {
	p, err := s.repo.QueryJob(ctx, q)
	if err != nil {
		return Page{}, err
	}
	var l []Summary
	for _, j := range p.Content {
		l = append(l, toSummary(j))
	}
	res := Page{Total: p.Total, Content: l}
	return res, nil
}

func (s mgrSvcImpl) CancelTask(ctx context.Context, thingId, jobId string, r CancelTaskReq, force bool) error {
	var t TaskEntity
	err := s.repo.ExecWithTx(func(txRepo Repo) error {
		l, err := txRepo.QueryTask(ctx, jobId, thingId, TaskPageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 1}})
		if err != nil {
			return err
		}
		if l.Total == 0 {
			return errors.WithMessage(model.ErrNotFound, "task")
		}
		t = l.Content[0]
		if !force && t.Status == TaskInProgress {
			return errors.Wrap(model.ErrInvalidParams, "task is in progress")
		}
		toUpdate := map[string]any{
			"force_canceled": force,
			"status":         TaskCanceled,
		}
		if r.Version > 0 && t.Version != r.Version {
			return errors.WithMessage(model.ErrVersionConflict,
				fmt.Sprintf("current version %d, expect version %d", t.Version, r.Version))
		}
		if r.StatusDetails != nil {
			if sdBuf, err := json.Marshal(*r.StatusDetails); err == nil {
				toUpdate["status_details"] = sdBuf
			} else {
				return errors.WithMessage(model.ErrInvalidParams, "statusDetails: "+err.Error())
			}
		}
		return txRepo.UpdateTask(ctx, t.TaskId, toUpdate)
	})
	if err != nil {
		return err
	}
	s.jobCenter.ReceiveMgrMsg(MgrMsg{
		Typ:  MgrTypeCancelTask,
		Data: MgrMsgCancelTask{JobId: jobId, TaskId: t.TaskId, Operation: t.Operation, Force: force},
	})
	return nil
}

func (s mgrSvcImpl) DeleteTask(ctx context.Context, thingId, jobId string, taskId int64, force bool) (*TaskEntity, error) {
	var en *TaskEntity
	err := s.repo.ExecWithTx(func(txRepo Repo) error {
		t, err := txRepo.GetTask(ctx, taskId)
		if err != nil {
			return err
		}
		if t == nil {
			return model.ErrNotFound
		}
		if t.ThingId != thingId {
			return errors.WithMessage(model.ErrInvalidParams,
				fmt.Sprintf("task %d is not belong to thing %q", taskId, thingId))
		}
		if t.JobId != jobId {
			return errors.WithMessage(model.ErrInvalidParams,
				fmt.Sprintf("task %d is not belong to job %q", taskId, jobId))
		}
		if !force && t.Status == TaskInProgress {
			return errors.Wrap(model.ErrInvalidParams, "task is in progress")
		}
		en = t
		return txRepo.DeleteTask(ctx, t.TaskId)
	})
	if err != nil {
		return nil, err
	}
	s.jobCenter.ReceiveMgrMsg(MgrMsg{
		Typ:  MgrTypeDeleteTask,
		Data: MgrMsgDeleteTask{JobId: jobId, TaskId: en.TaskId, Operation: en.Operation, Force: force},
	})
	return en, nil
}

func (s mgrSvcImpl) GetTask(ctx context.Context, thingId, jobId string, taskId int64) (*Task, error) {
	e, err := s.repo.GetTask(ctx, taskId)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	if e.JobId != thingId || e.ThingId != thingId {
		return nil, errors.WithMessage(
			model.ErrInvalidParams,
			fmt.Sprintf("task %d is not belong job %q or thing %q", taskId, jobId, thingId))
	}
	if e == nil {
		return nil, nil
	}
	t := toTask(*e)
	return &t, nil
}

func (s mgrSvcImpl) QueryTaskForThing(ctx context.Context, thingId string, q TaskPageQuery) (TaskPage, error) {
	return s.queryTask(ctx, "", thingId, q)
}

func (s mgrSvcImpl) QueryTaskForJob(ctx context.Context, jobId string, q TaskPageQuery) (TaskPage, error) {
	return s.queryTask(ctx, jobId, "", q)
}

func (s mgrSvcImpl) queryTask(ctx context.Context, jobId, thingId string, q TaskPageQuery) (TaskPage, error) {
	res, err := s.repo.QueryTask(ctx, jobId, thingId, q)
	if err != nil {
		return TaskPage{}, err
	}
	var l []TaskSummary
	for _, t := range res.Content {
		l = append(l, toTaskSummary(t))
	}
	p := TaskPage{Total: res.Total, Content: l}
	return p, nil
}
