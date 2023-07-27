package job

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
)

type Entity struct {
	JobId            string         `gorm:"primaryKey;size:64"`
	TargetConfig     datatypes.JSON `gorm:"NOT NULL;"`
	JobDoc           datatypes.JSON
	Description      string `gorm:"size:256"`
	Operation        string `gorm:"NOT NULL;"`
	SchedulingConfig datatypes.JSON
	RolloutConfig    datatypes.JSON
	RetryConfig      datatypes.JSON
	TimeoutConfig    datatypes.JSON

	Status         Status `gorm:"NOT NULL;default:WAITING;"`
	ForceCanceled  bool   `gorm:"NOT NULL; DEFAULT: 0"`
	ProcessDetails datatypes.JSON
	Comment        string `gorm:"size:256; NOT NULL; default: ''"`
	ReasonCode     string `gorm:"size:64; NOT NULL; default: ''"`

	StartedAt   *time.Time
	CompletedAt *time.Time
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
	ForceCanceled bool `gorm:"NOT NULL; DEFAULT: 0"`
	StatusDetails datatypes.JSON
	RetryAttempt  uint8 `gorm:"NOT NULL; DEFAULT: 0"`

	QueuedAt    time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	UpdatedAt   time.Time `gorm:"autoUpdateTime; NOT NULL"`
	CreatedAt   time.Time `gorm:"autoCreateTime; NOT NULL;"`

	Version int `gorm:"NOT NULL; DEFAULT: 1"`
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
	var roConfig []byte = nil
	var retryConf []byte = nil
	var timeoutConf []byte = nil
	if r.SchedulingConfig != nil {
		schConfig, err = json.Marshal(*r.SchedulingConfig)
		if err != nil {
			return Entity{}, errors.WithMessage(model.ErrInvalidParams, "field schedulingConfig: "+err.Error())
		}
	}
	if r.RolloutConfig != nil {
		roConfig, err = json.Marshal(*r.RolloutConfig)
		if err != nil {
			return Entity{}, errors.WithMessage(model.ErrInvalidParams, "filed rolloutConfig: "+err.Error())
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

	var jd []byte
	if r.JobDoc != nil {
		if jd, err = json.Marshal(r.JobDoc); err != nil {
			return Entity{}, errors.WithMessage(model.ErrInvalidParams, "field jobDoc: "+err.Error())
		}
	}

	e := Entity{
		JobId:        r.JobId,
		TargetConfig: targetConf,
		Operation:    r.Operation,
		Description:  r.Description,
		JobDoc:       jd,

		SchedulingConfig: schConfig,
		RolloutConfig:    roConfig,
		RetryConfig:      retryConf,
		TimeoutConfig:    timeoutConf,
	}

	return e, nil
}

func toTaskEntities(jobId, operation string, tgt TargetConfig) []TaskEntity {
	var l []TaskEntity
	for _, t := range tgt.Things {
		t := TaskEntity{
			JobId:     jobId,
			ThingId:   t,
			Operation: operation,
			QueuedAt:  time.Now(),
		}
		l = append(l, t)
	}
	return l
}

func toDetail(e Entity, tsc []TaskStatusCount) (Detail, error) {
	var targetConf TargetConfig
	var schConf SchedulingConfig
	var roConf RolloutConfig
	var retryConf RetryConfig
	var timeoutConf TimeoutConfig

	pd := ProcessDetails{}
	for _, sc := range tsc {
		switch sc.Status {
		case TaskQueued:
			pd.Queued = sc.Count
		case TaskSent:
			pd.Sent = sc.Count
		case TaskInProgress:
			pd.InProgress = sc.Count
		case TaskFailed:
			pd.Failed = sc.Count
		case TaskSucceeded:
			pd.Succeeded = sc.Count
		case TaskCanceled:
			pd.Canceled = sc.Count
		case TaskTimeOut:
			pd.TimedOut = sc.Count
		case TaskRejected:
			pd.TimedOut = sc.Count
		default:
			log.Errorf("unexpected task status %q", sc.Status)
		}
	}

	var jd map[string]any
	if len(e.JobDoc) != 0 {
		if err := json.Unmarshal(e.JobDoc, &jd); err != nil {
			return Detail{}, err
		}
	}

	d := Detail{
		JobId:          e.JobId,
		Operation:      e.Operation,
		Description:    e.Description,
		JobDoc:         jd,
		Status:         e.Status,
		ForceCanceled:  e.ForceCanceled,
		ProcessDetails: pd,
		Comment:        e.Comment,
		ReasonCode:     e.ReasonCode,
		UpdatedAt:      e.UpdatedAt.UnixMilli(),
		CreatedAt:      e.CreatedAt.UnixMilli(),
		Version:        e.Version,
	}

	d.StartedAt = timeToMs(e.StartedAt)
	d.CompletedAt = timeToMs(e.CompletedAt)

	if err := json.Unmarshal(e.TargetConfig, &targetConf); err != nil {
		return Detail{}, errors.WithMessage(model.ErrInternal, "field targetConfig in db")
	} else {
		d.TargetConfig = targetConf
	}

	if e.SchedulingConfig != nil {
		if err := json.Unmarshal(e.SchedulingConfig, &schConf); err != nil {
			return Detail{}, errors.WithMessage(model.ErrInternal, "field schedulingConfig in db")
		}
		d.SchedulingConfig = &schConf
	}
	if e.RolloutConfig != nil {
		if err := json.Unmarshal(e.RolloutConfig, &roConf); err != nil {
			return Detail{}, errors.WithMessage(model.ErrInternal, "field rolloutConfig in db")
		}
		d.RolloutConfig = &roConf
	}
	if e.RetryConfig != nil {
		if err := json.Unmarshal(e.RetryConfig, &retryConf); err != nil {
			return Detail{}, errors.WithMessage(model.ErrInternal, "field retryConfig in db")
		}
		d.RetryConfig = &retryConf
	}
	if e.TimeoutConfig != nil {
		if err := json.Unmarshal(e.TimeoutConfig, &timeoutConf); err != nil {
			return Detail{}, errors.WithMessage(model.ErrInternal, "field timeoutConfig in db")
		}
		d.TimeoutConfig = &timeoutConf
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
	s.StartedAt = timeToMs(e.StartedAt)
	s.CompletedAt = timeToMs(e.CompletedAt)
	return s
}

func toTasks(el []TaskEntity) []Task {
	var l []Task
	for _, e := range el {
		l = append(l, toTask(e))
	}
	return l
}

func toTask(e TaskEntity) Task {
	var stDetails StatusDetails
	t := Task{
		JobId:         e.JobId,
		TaskId:        e.TaskId,
		ThingId:       e.ThingId,
		Operation:     e.Operation,
		ForceCanceled: e.ForceCanceled,
		Status:        e.Status,
		Progress:      e.Progress,
		UpdatedAt:     e.UpdatedAt.UnixMilli(),
		Version:       e.Version,
	}

	t.QueuedAt = timeToMs(&e.QueuedAt)
	t.StartedAt = timeToMs(e.StartedAt)
	t.CompletedAt = timeToMs(e.CompletedAt)

	if e.StatusDetails != nil {
		err := json.Unmarshal(e.StatusDetails, &stDetails)
		if err != nil {
			log.Errorf("task entity statusDetails is invalid json, content=%s error: %v", e.StatusDetails, err)
		}
	}
	t.StatusDetails = stDetails
	return t
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
	ts := e.QueuedAt.UnixMilli()
	s.QueuedAt = &ts

	s.StartedAt = timeToMs(e.StartedAt)
	s.CompletedAt = timeToMs(e.CompletedAt)
	return s
}

func timeToMs(t *time.Time) *int64 {
	if t != nil {
		ts := t.UnixMilli()
		return &ts
	}
	return nil
}
