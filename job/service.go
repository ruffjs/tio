package job

import (
	"context"
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

type MgrService interface {
	// Job API

	CreateJob(ctx context.Context, r CreateReq) (Detail, error)
	UpdateJob(ctx context.Context, r UpdateReq) error
	CancelJob(ctx context.Context, r CancelReq, force bool) error
	DeleteJob(ctx context.Context, id string, force bool) error
	GetJob(ctx context.Context, id string) (Detail, error)
	QueryJob(ctx context.Context, q PageQuery) (Page, error)

	// Task API

	CancelTask(ctx context.Context, thingId, jobId string, r CancelTaskReq, force bool) error
	DeleteTask(ctx context.Context, thingId, jobId string, taskId int64, force bool) error
	GetTask(ctx context.Context, thingId, jobId string, taskId int64) (Task, error)
	QueryTaskForThing(ctx context.Context, thingId string, q TaskPageQuery) (TaskPage, error)
	QueryTaskForJob(ctx context.Context, jobId string, q TaskPageQuery) (TaskPage, error)
}

type Repo interface {
	// Job API

	CreateJob(ctx context.Context, j Entity) (Entity, error)
	UpdateJob(ctx context.Context, m map[string]any) error
	DeleteJob(ctx context.Context, id string, force bool) error
	GetJob(ctx context.Context, id string) (Entity, error)
	QueryJob(ctx context.Context, q PageQuery) (model.PageData[Entity], error)

	// Task API

	CreateTask(ctx context.Context, t TaskEntity) (TaskEntity, error)
	UpdateTask(ctx context.Context, taskId int64, m map[string]any) error
	DeleteTask(ctx context.Context, taskId int64) error
	GetTask(ctx context.Context, taskId int64) (TaskEntity, error)
	QueryTask(ctx context.Context, jobId, thingId string, q TaskPageQuery) (model.PageData[TaskEntity], error)
}

func NewMgrService(repo Repo, idProvider tio.IdProvider) MgrService {
	return mgrSvcImpl{repo, idProvider}
}

var _ MgrService = &mgrSvcImpl{}

type mgrSvcImpl struct {
	repo       Repo
	idProvider tio.IdProvider
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
	if d, err := toDetail(res); err != nil {
		return Detail{}, errors.WithMessage(model.ErrInternal, "to job detail")
	} else {
		return d, nil
	}
	// TODO: notify job runner
}

func (s mgrSvcImpl) UpdateJob(ctx context.Context, r UpdateReq) error {
	if err := r.valid(); err != nil {
		return err
	}
	// TODO update job by job runner
	panic("implement me")
}

func (s mgrSvcImpl) CancelJob(ctx context.Context, r CancelReq, force bool) error {
	if err := r.valid(); err != nil {
		return err
	}
	// TODO: do cancel job by job runner
	panic("implement me")
}

func (s mgrSvcImpl) DeleteJob(ctx context.Context, id string, force bool) error {
	// TODO: do delete job by job runner
	panic("implement me")
}

func (s mgrSvcImpl) GetJob(ctx context.Context, id string) (Detail, error) {
	e, err := s.repo.GetJob(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	d, err := toDetail(e)
	if err != nil {
		return Detail{}, err
	}
	return d, nil
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
	// TODO: do cancel task by job runner
	panic("implement me")
}

func (s mgrSvcImpl) DeleteTask(ctx context.Context, thingId, jobId string, taskId int64, force bool) error {
	// TODO: do delete task by job runner
	panic("implement me")
}

func (s mgrSvcImpl) GetTask(ctx context.Context, thingId, jobId string, taskId int64) (Task, error) {
	e, err := s.repo.GetTask(ctx, taskId)
	if err != nil {
		return Task{}, err
	}
	if e.JobId != thingId || e.ThingId != thingId {
		return Task{}, errors.WithMessage(
			model.ErrInvalidParams,
			fmt.Sprintf("task %d is not belong job %q or thing %q", taskId, jobId, thingId))
	}
	if t, err := toTask(e); err != nil {
		return Task{}, err
	} else {
		return t, nil
	}
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
