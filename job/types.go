package job

import "errors"

// Types for Job and Task
// Tasks arise from Job, and the operation of Job consists of Tasks specific to each Thing

// The operation retained by the system starts with $
const (
	SysOpDirectMethod = "$directMethod"
	SysOpUpdateShadow = "$updateShadow"
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

func (TaskStatus) Values() []string {
	return []string{
		string(TaskQueued), string(TaskInProgress),
		string(TaskFailed), string(TaskSucceeded),
		string(TaskRejected), string(TaskTimeOut),
		string(TaskCanceled), string(TaskRemoved),
	}
}

var ErrUnknownStatus = errors.New("unknown task status")

func (s TaskStatus) Of(value string) (TaskStatus, error) {
	l := s.Values()
	for _, v := range l {
		if v == value {
			return TaskStatus(v), nil
		}
	}
	return "", ErrUnknownStatus
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
	QueuedAt      int64         `json:"queuedAt"` // timestamp in ms
	StartedAt     int64         `json:"startedAt"`
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

	QueuedAt  int64 `json:"queuedAt"` // timestamp in ms
	StartedAt int64 `json:"startedAt"`
	UpdatedAt int64 `json:"updatedAt"`

	Version int `json:"version"`
}

// The following data types are used by management and control applications to communicate with to Jobs.

type Status string

const (
	StatusScheduled  Status = "SCHEDULED"
	StatusInProgress Status = "IN_PROGRESS"
	StatusCanceled   Status = "CANCELED"
	StatusSucceeded  Status = "SUCCEEDED"
)

func (Status) Values() []string {
	return []string{
		string(StatusScheduled), string(StatusInProgress),
		string(StatusCanceled), string(StatusSucceeded),
	}
}

func (s Status) Of(value string) (Status, error) {
	l := s.Values()
	for _, v := range l {
		if v == value {
			return Status(v), nil
		}
	}
	return "", ErrUnknownStatus
}

type MaintenanceWindow struct {
	StartTime         string `json:"startTime"` //  cron, eg: "cron(0 0 18 ? * MON *)" means "every monday at 18:00"
	DurationInMinutes int    `json:"durationInMinutes"`
}
type SchedulingConfig struct {
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	EndBehavior string `json:"endBehavior"`

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
	InProgressMinutes int64 `json:"inProgressMinutes"` // max time for taskutions stay in "IN_PROGRESS" status
}

type ProcessDetails struct {
	ProcessingTargets []string // The target things to which the job taskution is being rolled out

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

	TargetConfig     TargetConfig     `json:"targetConfig"`
	JobDoc           string           `json:"jobDoc"`
	Description      string           `json:"description"`
	Operation        string           `json:"operation"`
	SchedulingConfig SchedulingConfig `json:"schedulingConfig"`
	RetryConfig      RetryConfig      `json:"retryConfig"`
	TimeoutConfig    TimeoutConfig    `json:"timeoutConfig"`

	Status         Status         `json:"status"`
	ForceCanceled  bool           `json:"forceCanceled"`
	ProcessDetails ProcessDetails `json:"processDetails"`
	Comment        string         `json:"comment"`
	ReasonCode     string         `json:"reasonCode"`

	StartedAt   int64 `json:"startedAt"`
	CompletedAt int64 `json:"completedAt"`
	UpdatedAt   int64 `json:"updatedAt"`

	Version int `json:"version"`
}

type Summary struct {
	JobId     string `json:"jobId"`
	Operation string `json:"operation"`

	Status      Status `json:"status"`
	StartedAt   int64  `json:"startedAt"`
	CompletedAt int64  `json:"completedAt"`
	UpdatedAt   int64  `json:"updatedAt"`

	Version int `json:"version"`
}

type Task struct {
	JobId     string `json:"jobId"`
	TaskId    int64  `json:"taskId"`
	Operation string `json:"operation"`

	ForceCanceled bool          `json:"forceCanceled"`
	Status        TaskStatus    `json:"status"`
	StatusDetails StatusDetails `json:"statusDetails"`
	Progress      int           `json:"progress"`
	QueuedAt      int64         `json:"queuedAt"`
	StartedAt     int64         `json:"startedAt"`
	UpdatedAt     int64         `json:"updatedAt"`

	Version int `json:"version"`
}

type TaskSummary struct {
	TaskId    int64  `json:"taskId"`
	Operation string `json:"operation"`

	RetryAttempt uint8      `json:"retryAttempt"`
	Status       TaskStatus `json:"status"`
	Progress     int        `json:"progress"`
	QueuedAt     int64      `json:"queuedAt"`
	StartedAt    int64      `json:"startedAt"`
	UpdatedAt    int64      `json:"updatedAt"`
}

type TaskSummaryForJob struct {
	ThingId     string      `json:"thingId"`
	TaskSummary TaskSummary `json:"taskSummary"`
}

type TaskSummaryForThing struct {
	JobId       string      `json:"jobId"`
	TaskSummary TaskSummary `json:"taskSummary"`
}

// The following is used by http api request body

type CreateParameters struct {
	JobId            string       `json:"jobId"` // optional
	TargetConfig     TargetConfig `json:"targetConfig"`
	Operation        string       `json:"operation"`        // optional
	JobDoc           string       `json:"jobDoc"`           // optional
	Description      string       `json:"description"`      // optional
	SchedulingConfig any          `json:"schedulingConfig"` // optional
	RetryConfig      any          `json:"retryConfig"`      // optional, taskutions retry config
	TimeoutConfig    any          `json:"timeoutConfig"`    // optional
}

type UpdateParameters struct {
	Description   string `json:"description"`   // optional
	RetryConfig   any    `json:"retryConfig"`   // optional
	TimeoutConfig any    `json:"timeoutConfig"` // optional
}

type CancelParameters struct {
	Comment    string `json:"comment"`    // optional
	ReasonCode string `json:"reasonCode"` // optional
}

type CancelTaskParameters struct {
	Version       int           `json:"version"`       // optional, expected version
	StatusDetails StatusDetails `json:"statusDetails"` // optional
}
