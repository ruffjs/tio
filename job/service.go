package job

import (
	"context"
	"ruff.io/tio/pkg/model"
)

type PageQuery struct {
	Status Status `json:"status"`
	model.PageQuery
}

type Page model.PageData[Summary]

type TaskPageQuery struct {
	Status TaskStatus `json:"status"`
	model.PageQuery
}
type TaskPageForThing model.PageData[TaskSummaryForThing]
type TaskPageForJob model.PageData[TaskSummaryForJob]

type Service interface {
	// Job API

	CreateJob(ctx context.Context, p CreateParameters) (IdResp, error)
	UpdateJob(ctx context.Context, p UpdateParameters) error
	CancelJob(ctx context.Context, p CancelParameters, force bool) error
	DeleteJob(ctx context.Context, id string, force bool) error
	GetJob(ctx context.Context, id string) (Detail, error)
	QueryJob(ctx context.Context, q PageQuery) (Page, error)

	// Task API

	CancelTask(ctx context.Context, thingId, jobId, taskId string, force bool) error
	DeleteTask(ctx context.Context, thingId, jobId, taskId string, force bool) error
	GetTask(ctx context.Context, thingId, jobId, taskId string) (Task, error)
	QueryTaskForThing(ctx context.Context, q TaskPageQuery) (TaskPageForThing, error)
	QueryTaskForJob(ctx context.Context, q TaskPageQuery) (TaskPageForJob, error)
}
