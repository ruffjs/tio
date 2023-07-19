package job

import (
	"github.com/pkg/errors"
	"ruff.io/tio/shadow"
	"time"
)

// Types for Job and Task
// Tasks arise from Job, and the operation of Job consists of Tasks specific to each Thing

// The operation retained by the system starts with $
const (
	SysOpDirectMethod = "$directMethod"
	SysOpUpdateShadow = "$updateShadow"

	TargetTypeThingId = "THING_ID"
	TargetTypeGroup   = "GROUP"
)

type StatusDetails map[string]any

// The following data types are used to communicate with the tio Jobs service over the MQTT

type TaskStatus string

const (
	TaskQueued     TaskStatus = "QUEUED"
	TaskInProgress TaskStatus = "IN_PROGRESS"
	TaskFailed     TaskStatus = "FAILED"
	TaskSucceeded  TaskStatus = "SUCCEEDED"
	TaskCanceled   TaskStatus = "CANCELED"
	TaskTimeOut    TaskStatus = "TIMED_OUT"
	TaskRejected   TaskStatus = "REJECTED"
	TaskRemoved    TaskStatus = "REMOVED"
)

var taskStatusValues = []string{
	string(TaskQueued), string(TaskInProgress),
	string(TaskFailed), string(TaskSucceeded),
	string(TaskRejected), string(TaskTimeOut),
	string(TaskCanceled), string(TaskRemoved),
}

func (TaskStatus) Values() []string {
	return taskStatusValues
}

func (s TaskStatus) String() string {
	return string(s)
}

var ErrUnknownEnum = errors.New("unknown enum")

func (s TaskStatus) Of(value string) (TaskStatus, error) {
	l := s.Values()
	for _, v := range l {
		if v == value {
			return TaskStatus(v), nil
		}
	}
	return "", errors.WithMessage(ErrUnknownEnum, "TaskStatus")
}

type TTask struct {
	JobId     string `json:"jobId"`
	ThingId   string `json:"thingId"`
	TaskId    int64  `json:"taskId"`
	JobDoc    string `json:"jobDoc"`
	Operation string `json:"operation"`

	Status        TaskStatus    `json:"status"`
	StatusDetails StatusDetails `json:"statusDetails"`
	Progress      int           `json:"progress"` // 0 - 100
	QueuedAt      *int64        `json:"queuedAt"` // timestamp in ms
	StartedAt     *int64        `json:"startedAt"`
	UpdatedAt     int64         `json:"updatedAt"`

	Version int `json:"version"`
}

type TTaskState struct {
	Status        TaskStatus    `json:"status"`
	StatusDetails StatusDetails `json:"statusDetails"`
	Progress      int           `json:"progress"` // 0 - 100
	Version       int           `json:"version"`
}

type TTaskSummary struct {
	JobId     string `json:"jobId"`
	TaskId    int64  `json:"taskId"`
	Operation string `json:"operation"`

	QueuedAt  *int64 `json:"queuedAt"` // timestamp in ms
	StartedAt *int64 `json:"startedAt"`
	UpdatedAt int64  `json:"updatedAt"`

	Version int `json:"version"`
}

// The following types are the concrete types for the interaction between Thing and Job, witch contain types above.

type TTasksNotify struct {
	Tasks     []TTaskSummary `json:"tasks"`
	Timestamp int64          `json:"timestamp"`
}

type TTaskNotifyNext struct {
	Task      TTask `json:"task"`
	Timestamp int64 `json:"timestamp"`
}

type TPendingTasksResp struct {
	InProgressTasks []TTaskSummary `json:"inProgressTasks"`
	QueuedTasks     []TTaskSummary `json:"queuedTasks"`
	Timestamp       int64          `json:"timestamp"`
	ClientToken     string         `json:"clientToken"`
}

type TStartNextPendingTaskReq struct {
	StatusDetails StatusDetails `json:"statusDetails"`
	ClientToken   string        `json:"clientToken"`
}

type TGetTaskReq struct {
	JobId         string `json:"jobId"`
	ThingId       string `json:"thingId"`
	TaskId        int64  `json:"taskId"`
	IncludeJobDoc bool   `json:"includeJobDoc"`
	ClientToken   string `json:"clientToken"`
}

type TGetTaskResp struct {
	Task        TTask  `json:"task"`
	Timestamp   int64  `json:"timestamp"`
	ClientToken string `json:"clientToken"`
}

type TUpdateTaskReq struct {
	TaskId        int64         `json:"taskId"`
	Status        TaskStatus    `json:"status"`
	StatusDetails StatusDetails `json:"statusDetails"`
	Progress      int           `json:"progress"` // 0 - 100
	Version       int           `json:"version"`

	IncludeJobDoc    bool   `json:"includeJobDoc"`
	IncludeTaskState bool   `json:"includeTaskState"`
	ClientToken      string `json:"clientToken"`
}

type TUpdateTaskResp struct {
	TaskState   TTaskState `json:"taskState"`
	JobDoc      string     `json:"jobDoc"`
	Timestamp   int64      `json:"timestamp"`
	ClientToken string     `json:"clientToken"`
}

// The following data types are used by management and control applications to communicate with to Jobs.

type Status string

const (
	StatusScheduled          Status = "SCHEDULED"
	StatusInProgress         Status = "IN_PROGRESS"
	StatusCanceled           Status = "CANCELED"
	StatusCompleted          Status = "COMPLETED"
	StatusDeletionInProgress Status = "DELETION_IN_PROGRESS"
)

var statusScheduledValues = []string{
	string(StatusScheduled), string(StatusInProgress),
	string(StatusCanceled), string(StatusCompleted),
	string(StatusDeletionInProgress),
}

func (Status) Values() []string {
	return statusScheduledValues
}
func (s Status) String() string {
	return string(s)
}
func (s Status) Of(value string) (Status, error) {
	l := s.Values()
	for _, v := range l {
		if v == value {
			return Status(v), nil
		}
	}
	return "", errors.WithMessagef(ErrUnknownEnum, "Status")
}

type ScheduleEndBehavior string

const (
	ScheduleEndBehaviorStopRollout ScheduleEndBehavior = "STOP_ROLLOUT"
	ScheduleEndBehaviorCancel      ScheduleEndBehavior = "CANCEL"
	ScheduleEndBehaviorForceCancel ScheduleEndBehavior = "FORCE_CANCEL"
)

var scheduleEndBehaviorValues = []string{
	string(ScheduleEndBehaviorStopRollout),
	string(ScheduleEndBehaviorCancel),
	string(ScheduleEndBehaviorForceCancel),
}

func (ScheduleEndBehavior) Values() []string {
	return scheduleEndBehaviorValues
}
func (b ScheduleEndBehavior) String() string {
	return string(b)
}
func (b ScheduleEndBehavior) Of(value string) (ScheduleEndBehavior, error) {
	l := b.Values()
	for _, v := range l {
		if v == value {
			return ScheduleEndBehavior(v), nil
		}
	}
	return "", errors.Wrap(ErrUnknownEnum, "ScheduleEndBehavior")
}

type MaintenanceWindow struct {
	StartTime         string `json:"startTime"` //  cron, eg: "cron(0 0 18 ? * MON *)" means "every monday at 18:00"
	DurationInMinutes int    `json:"durationInMinutes"`
}
type SchedulingConfig struct {
	StartTime   time.Time           `json:"startTime"`   // ISO-8601 date time
	EndTime     *time.Time          `json:"endTime"`     // optional, ISO8601 date time
	EndBehavior ScheduleEndBehavior `json:"endBehavior"` // STOP_ROLLOUT | CANCEL | FORCE_CANCEL

	// To be implemented later
	//MaintenanceWindows []MaintenanceWindow `json:"maintenanceWindows"`
}

type RetryConfig struct {
	CriteriaList []RetryConfigItem
}
type RetryConfigItem struct {
	FailureType     string `json:"failureType"` // FAILED | TIMED_OUT | ALL
	NumberOfRetries int    `json:"numberOfRetries"`
}

type TimeoutConfig struct {
	InProgressMinutes int `json:"inProgressMinutes"` // max time for task stay in "IN_PROGRESS" status
}

type ProcessDetails struct {
	ProcessingTargets []string // The target things to which the job task is being rolled out

	// Status statistics with Thing as the statistical unit

	Canceled   int `json:"canceled"`
	Failed     int `json:"failed"`
	InProgress int `json:"inProgress"`
	Queued     int `json:"queued"`
	Rejected   int `json:"rejected"`
	Removed    int `json:"removed"`
	Succeeded  int `json:"succeeded"`
	TimedOut   int `json:"timedOut"`
}

type TargetConfig struct {
	Type   string   `json:"type"` // "THING_ID". Or can be "GROUP" in future ?
	Things []string `json:"things"`
}

type Detail struct {
	JobId string `json:"jobId"`

	TargetConfig     TargetConfig      `json:"targetConfig"`
	JobDoc           string            `json:"jobDoc"`
	Description      string            `json:"description"`
	Operation        string            `json:"operation"`
	SchedulingConfig *SchedulingConfig `json:"schedulingConfig"`
	RetryConfig      *RetryConfig      `json:"retryConfig"`
	TimeoutConfig    *TimeoutConfig    `json:"timeoutConfig"`

	Status         Status         `json:"status"`
	ForceCanceled  bool           `json:"forceCanceled"`
	ProcessDetails ProcessDetails `json:"processDetails"`
	Comment        string         `json:"comment"`
	ReasonCode     string         `json:"reasonCode"`

	StartedAt   *int64 `json:"startedAt"`
	CompletedAt *int64 `json:"completedAt"`
	UpdatedAt   int64  `json:"updatedAt"`
	CreatedAt   int64  `json:"createdAt"`

	Version int `json:"version"`
}

type Summary struct {
	JobId     string `json:"jobId"`
	Operation string `json:"operation"`

	Status      Status `json:"status"`
	StartedAt   *int64 `json:"startedAt"`
	CompletedAt *int64 `json:"completedAt"`
	UpdatedAt   int64  `json:"updatedAt"`
	CreatedAt   int64  `json:"createdAt"`

	Version int `json:"version"`
}

type Task struct {
	JobId     string `json:"jobId"`
	TaskId    int64  `json:"taskId"`
	Operation string `json:"operation"`

	ForceCanceled bool          `json:"forceCanceled"`
	Status        TaskStatus    `json:"status"`
	StatusDetails StatusDetails `json:"statusDetails"`
	Progress      uint8         `json:"progress"`
	QueuedAt      *int64        `json:"queuedAt"`
	StartedAt     *int64        `json:"startedAt"`
	UpdatedAt     int64         `json:"updatedAt"`
	CreatedAt     int64         `json:"createdAt"`

	Version int `json:"version"`
}

type TaskSummary struct {
	TaskId    int64  `json:"taskId"`
	JobId     string `json:"jobId"`
	ThingId   string `json:"thingId"`
	Operation string `json:"operation"`

	RetryAttempt uint8      `json:"retryAttempt"`
	Status       TaskStatus `json:"status"`
	Progress     uint8      `json:"progress"`
	QueuedAt     *int64     `json:"queuedAt"`
	StartedAt    *int64     `json:"startedAt"`
	UpdatedAt    int64      `json:"updatedAt"`
	CreatedAt    int64      `json:"createdAt"`
}

// The following is used by http api request body

type CreateReq struct {
	JobId        string       `json:"jobId"` // optional
	TargetConfig TargetConfig `json:"targetConfig"`
	Operation    string       `json:"operation"`
	Description  string       `json:"description"` // optional

	// JobDoc optional, when operation is "$updateShadow" or "$updateShadow",
	// job doc should be json string of UpdateShadowReq or InvokeDirectMethodReq
	JobDoc string `json:"jobDoc"`

	SchedulingConfig *SchedulingConfig `json:"schedulingConfig"` // optional
	RetryConfig      *RetryConfig      `json:"retryConfig"`      // optional, tasks retry config
	TimeoutConfig    *TimeoutConfig    `json:"timeoutConfig"`    // optional
}
type UpdateShadowReq struct {
	State struct {
		Desired shadow.StateValue `json:"desired"`
	} `json:"state"`
}
type InvokeDirectMethodReq struct {
	Method      string `json:"method"`
	ConnTimeout int    `json:"connTimeout"`     // in second
	RespTimeout int    `json:"responseTimeout"` // in second
	Payload     any    `json:"payload"`
}

type UpdateReq struct {
	Description   *string        `json:"description"`   // optional
	RetryConfig   *RetryConfig   `json:"retryConfig"`   // optional
	TimeoutConfig *TimeoutConfig `json:"timeoutConfig"` // optional
}

type CancelReq struct {
	Comment    *string `json:"comment"`    // optional
	ReasonCode *string `json:"reasonCode"` // optional
}

type CancelTaskReq struct {
	Version       int            `json:"version"`       // optional, expected version
	StatusDetails *StatusDetails `json:"statusDetails"` // optional
}

type IdResp struct {
	JobId string `json:"jobId"`
}
