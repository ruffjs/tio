package job

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"ruff.io/tio/pkg/model"
	"strings"
	"time"
)

type Entity struct {
	JobId            string         `gorm:"primaryKey;size:64"`
	TargetConfig     datatypes.JSON `gorm:"NOT NULL;"`
	JobDoc           string         `gorm:"type:text; NOT NULL;"`
	Description      string         `gorm:"size:256"`
	Operation        string         `gorm:"NOT NULL;"`
	SchedulingConfig datatypes.JSON
	RetryConfig      datatypes.JSON
	TimeoutConfig    datatypes.JSON

	Status         Status `gorm:"NOT NULL;default:SCHEDULED;"`
	ForceCanceled  bool   `gorm:"NOT NULL; DEFAULT: 0"`
	ProcessDetails datatypes.JSON
	Comment        string `gorm:"size:256"`
	ReasonCode     string `gorm:"size:64"`

	StartedAt   time.Time
	CompletedAt time.Time
	UpdatedAt   time.Time `gorm:"autoUpdateTime; NOT NULL;"`
	CreatedAt   time.Time `gorm:"autoCreateTime; NOT NULL;"`

	Version int `gorm:"NOT NULL; DEFAULT: 1"`

	Tasks []TaskEntity `gorm:"foreignKey:JobId;constraint:OnDelete:CASCADE"`
}

func (Entity) TableName() string {
	return "job"
}

func (Status) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "mysql":
		return fmt.Sprintf("enum('%s')", strings.Join(Status.Values(""), "','"))
	case "sqlite":
		return "text"
	}
	return ""
}

type TaskEntity struct {
	TaskId  int64  `gorm:"primaryKey;size:64"`
	JobId   string `gorm:"size:64; NOT NULL;"`
	ThingId string `gorm:"size:64; NOT NULL;"`

	Operation     string     `gorm:"size:64; NOT NULL;"`
	Status        TaskStatus `gorm:"NOT NULL;default:QUEUED;"`
	Progress      uint8
	ForceCanceled bool           `gorm:"NOT NULL; DEFAULT: 0"`
	StatusDetails datatypes.JSON `gorm:"NOT NULL;"`
	RetryAttempt  uint8          `gorm:"NOT NULL; DEFAULT: 0"`

	QueuedAt  time.Time
	StartedAt time.Time
	UpdatedAt time.Time `gorm:"autoUpdateTime; NOT NULL"`
	CreatedAt time.Time `gorm:"autoCreateTime; NOT NULL;"`

	Version int `gorm:"NOT NULL; DEFAULT: 1"`

	//Job Entity `gorm:"foreignKey:JobId;references:JobId"`
	//Job Entity `gorm:"foreignKey:job_id; references: job_id, constraint:OnDelete:CASCADE;"`
}

func (t TaskEntity) TableName() string {
	return "job_task"
}
func (TaskStatus) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "mysql":
		return fmt.Sprintf("enum('%s')", strings.Join(TaskStatus.Values(""), "','"))
	case "sqlite":
		return "text"
	}
	return ""
}

func toEntity(r CreateReq) (Entity, error) {
	targetConf, err := json.Marshal(r.TargetConfig)
	if err != nil {
		return Entity{}, errors.WithMessage(model.ErrInvalidParams, "field targetConfig: "+err.Error())
	}
	var schConfig []byte = nil
	var retryConf []byte = nil
	var timeoutConf []byte = nil
	if r.SchedulingConfig != nil {
		schConfig, err = json.Marshal(*r.SchedulingConfig)
		if err != nil {
			return Entity{}, errors.WithMessage(model.ErrInvalidParams, "field schedulingConfig: "+err.Error())
		}
	}
	if r.RetryConfig != nil {
		if retryConf, err = json.Marshal(*r.RetryConfig); err != nil {
			return Entity{}, errors.WithMessage(model.ErrInvalidParams, "field retryConfig: "+err.Error())
		}
	}
	if r.TimeoutConfig != nil {
		if timeoutConf, err = json.Marshal(*r.TimeoutConfig); err != nil {
			return Entity{}, errors.WithMessage(model.ErrInvalidParams, "field timeoutConfig: "+err.Error())
		}
	}

	e := Entity{
		JobId:        r.JobId,
		TargetConfig: targetConf,
		Operation:    r.Operation,
		Description:  r.Description,
		JobDoc:       r.JobDoc,

		SchedulingConfig: schConfig,
		RetryConfig:      retryConf,
		TimeoutConfig:    timeoutConf,
	}

	return e, nil
}

func toDetail(e Entity) (Detail, error) {
	var targetConf TargetConfig
	var schConf SchedulingConfig
	var retryConf RetryConfig
	var timeoutConf TimeoutConfig
	//var procDetails ProcessDetails = ProcessDetails{} // TODO

	d := Detail{
		JobId:         e.JobId,
		Operation:     e.Operation,
		Description:   e.Description,
		JobDoc:        e.JobDoc,
		Status:        e.Status,
		ForceCanceled: e.ForceCanceled,
		Comment:       e.Comment,
		ReasonCode:    e.ReasonCode,
		UpdatedAt:     e.UpdatedAt.UnixMilli(),
		CreatedAt:     e.CreatedAt.UnixMilli(),
		Version:       e.Version,
	}

	if !e.StartedAt.IsZero() {
		ts := e.StartedAt.UnixMilli()
		d.StartedAt = &ts
	}
	if !e.CompletedAt.IsZero() {
		ts := e.CompletedAt.UnixMilli()
		d.CompletedAt = &ts
	}

	if err := json.Unmarshal(e.TargetConfig, &targetConf); err != nil {
		return Detail{}, errors.WithMessage(model.ErrInternal, "field targetConfig in db")
	} else {
		d.TargetConfig = targetConf
	}

	if e.SchedulingConfig != nil {
		if err := json.Unmarshal(e.SchedulingConfig, &schConf); err != nil {
			return Detail{}, errors.WithMessage(model.ErrInternal, "field schedulingConfig in db")
		} else {
			d.SchedulingConfig = &schConf
		}
	}
	if e.RetryConfig != nil {
		if err := json.Unmarshal(e.RetryConfig, &retryConf); err != nil {
			return Detail{}, errors.WithMessage(model.ErrInternal, "field retryConfig in db")
		} else {
			d.RetryConfig = &retryConf
		}
	}
	if e.TimeoutConfig != nil {
		if err := json.Unmarshal(e.TimeoutConfig, &timeoutConf); err != nil {
			return Detail{}, errors.WithMessage(model.ErrInternal, "field timeoutConfig in db")
		} else {
			d.TimeoutConfig = &timeoutConf
		}
	}

	return d, nil
}

func toSummary(e Entity) Summary {
	s := Summary{
		JobId:     e.JobId,
		Operation: e.Operation,
		Status:    e.Status,
		UpdatedAt: e.UpdatedAt.UnixMilli(),
		Version:   e.Version,
	}

	if !e.StartedAt.IsZero() {
		ts := e.StartedAt.UnixMilli()
		s.StartedAt = &ts
	}
	if !e.CompletedAt.IsZero() {
		ts := e.CompletedAt.UnixMilli()
		s.CompletedAt = &ts
	}
	return s
}

func toTask(e TaskEntity) (Task, error) {
	var stDetails StatusDetails
	t := Task{
		JobId:         e.JobId,
		TaskId:        e.TaskId,
		Operation:     e.Operation,
		ForceCanceled: e.ForceCanceled,
		Status:        e.Status,
		Progress:      e.Progress,
		UpdatedAt:     e.UpdatedAt.UnixMilli(),
		Version:       e.Version,
	}

	if !e.QueuedAt.IsZero() {
		ts := e.QueuedAt.UnixMilli()
		t.QueuedAt = &ts
	}
	if !e.StartedAt.IsZero() {
		ts := e.StartedAt.UnixMilli()
		t.StartedAt = &ts
	}

	if e.StatusDetails != nil {
		err := json.Unmarshal(e.StatusDetails, &stDetails)
		if err != nil {
			return Task{}, errors.WithMessage(model.ErrInternal, "field statusDetails")
		}
	}
	t.StatusDetails = stDetails
	return t, nil
}

func toTaskSummary(e TaskEntity) TaskSummary {
	s := TaskSummary{
		TaskId:       e.TaskId,
		JobId:        e.JobId,
		ThingId:      e.ThingId,
		Operation:    e.Operation,
		RetryAttempt: e.RetryAttempt,
		Status:       e.Status,
		Progress:     e.Progress,
		UpdatedAt:    e.UpdatedAt.UnixMilli(),
		CreatedAt:    e.CreatedAt.UnixMilli(),
	}

	if !e.QueuedAt.IsZero() {
		ts := e.QueuedAt.UnixMilli()
		s.QueuedAt = &ts
	}
	if !e.StartedAt.IsZero() {
		ts := e.StartedAt.UnixMilli()
		s.StartedAt = &ts
	}
	return s
}
