package job

import (
	"fmt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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
	TaskId  uint64 `gorm:"primaryKey;size:64"`
	JobId   string `gorm:"size:64; NOT NULL;"`
	ThingId string `gorm:"size:64; NOT NULL;"`

	Operation     string     `gorm:"size:64; NOT NULL;"`
	Status        TaskStatus `gorm:"NOT NULL;default:QUEUED;"`
	Progress      uint8
	ForceCanceled bool `gorm:"NOT NULL; DEFAULT: 0"`
	StatusDetails datatypes.JSON

	QueuedAt  time.Time
	StartedAt time.Time
	UpdatedAt time.Time `gorm:"autoUpdateTime; NOT NULL"`

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
