package job

// The following data types are used to communicate with the tio Jobs service over the MQTT
// Their names start with T which means thing.

type ExecutionStatus string

const (
	ExecQueued     ExecutionStatus = "QUEUED"
	ExecInProgress ExecutionStatus = "IN_PROGRESS"
	ExecFailed     ExecutionStatus = "FAILED"
	ExecSucceeded  ExecutionStatus = "SUCCEEDED"
	ExecCanceled   ExecutionStatus = "CANCELED"
	ExecTimeOut    ExecutionStatus = "TIMED_OUT"
	ExecRejected   ExecutionStatus = "REJECTED"
	ExecRemoved    ExecutionStatus = "REMOVED"
)

type TExecution struct {
	JobId         string            `json:"jobId"`
	ThingId       string            `json:"thingId"`
	JobDocument   string            `json:"jobDocument"`
	Priority      uint8             `json:"priority"` // 1-10
	Operation     string            `json:"operation"`
	Status        ExecutionStatus   `json:"status"`
	StatusDetails map[string]string `json:"statusDetails"`
	QueuedAt      int64             `json:"queuedAt"` // timestamp in ms
	StartedAt     int64             `json:"startedAt"`
	UpdatedAt     int64             `json:"updatedAt"`
	Version       int               `json:"version"`
	ExecutionId   int64             `json:"executionId"`
}

type TExecutionState struct {
	Status        ExecutionStatus   `json:"status"`
	StatusDetails map[string]string `json:"statusDetails"`
	Version       int               `json:"version"`
}

type TExecutionSummary struct {
	JobId       string `json:"jobId"`
	ExecutionId int64  `json:"executionId"`
	Priority    uint8  `json:"priority"` // 1-10
	Operation   string `json:"operation"`
	QueuedAt    int64  `json:"queuedAt"` // timestamp in ms
	StartedAt   int64  `json:"startedAt"`
	UpdatedAt   int64  `json:"updatedAt"`
	Version     int    `json:"version"`
}

// The following data types are used by management and control applications to communicate with to Jobs.

// Status Job status
type Status string

const (
	StatusInProgress Status = "IN_PROGRESS"
	StatusCanceled   Status = "CANCELED"
	StatusSucceeded  Status = "SUCCEEDED"
	StatusScheduled  Status = "SCHEDULED"
)

type MaintenanceWindow struct {
	StartTime         string `json:"startTime"` //  cron
	DurationInMinutes int    `json:"durationInMinutes"`
}
type SchedulingConfig struct {
	StartTime          string              `json:"startTime"`
	EndTime            string              `json:"endTime"`
	EndBehavior        string              `json:"endBehavior"`
	MaintenanceWindows []MaintenanceWindow `json:"maintenanceWindows"`
}

type ExecutionsRetryConfig struct {
	CriteriaList []RetryConfigItem
}
type RetryConfigItem struct {
	FailureType     string `json:"failureType"` // FAILED | TIMED_OUT | ALL
	NumberOfRetries int    `json:"numberOfRetries"`
}

type TimeoutConfig struct {
	InProgressTimeoutInMinutes int64 `json:"inProgressTimeoutInMinutes"`
}

type ProcessDetails struct {
	ProcessingTargets       []string // The target things to which the job execution is being rolled out
	CountOfCanceledThings   int      `json:"countOfCanceledThings"`
	CountOfFailedThings     int      `json:"countOfFailedThings"`
	CountOfInProgressThings int      `json:"countOfInProgressThings"`
	CountOfQueuedThings     int      `json:"countOfQueuedThings"`
	CountOfRejectedThings   int      `json:"countOfRejectedThings"`
	CountOfRemovedThings    int      `json:"countOfRemovedThings"`
	CountOfSucceededThings  int      `json:"countOfSucceededThings"`
	CountOfTimedOutThings   int      `json:"countOfTimedOutThings"`
}

type Detail struct {
	JobId string `json:"jobId"`

	Targets               []string              `json:"targets"`
	Document              string                `json:"document"`
	Description           string                `json:"description"`
	Priority              uint8                 `json:"priority"`
	Operation             string                `json:"operation"`
	SchedulingConfig      SchedulingConfig      `json:"schedulingConfig"`
	ExecutionsRetryConfig ExecutionsRetryConfig `json:"executionsRetryConfig"`
	TimeoutConfig         TimeoutConfig         `json:"timeoutConfig"`

	Status         Status         `json:"status"`
	ForceCanceled  bool           `json:"forceCanceled"`
	ProcessDetails ProcessDetails `json:"jobProcessDetails"`
	Comment        string         `json:"comment"`
	ReasonCode     string         `json:"reasonCode"`

	StartedAt   int64 `json:"startedAt"`
	CompletedAt int64 `json:"completedAt"`
	UpdatedAt   int64 `json:"updatedAt"`

	Version int `json:"version"`
}

type Summary struct {
	JobId       string `json:"jobId"`
	Priority    uint8  `json:"priority"`
	Operation   string `json:"operation"`
	Status      Status `json:"status"`
	StartedAt   int64  `json:"startedAt"`
	CompletedAt int64  `json:"completedAt"`
	UpdatedAt   int64  `json:"updatedAt"`
	Version     int    `json:"version"`
}

type Execution struct {
	JobId         string            `json:"jobId"`
	ExecutionId   int64             `json:"executionId"`
	Priority      uint8             `json:"priority"`
	Operation     string            `json:"operation"`
	ForceCanceled bool              `json:"forceCanceled"`
	Status        ExecutionStatus   `json:"status"`
	StatusDetails map[string]string `json:"statusDetails"`
	QueuedAt      int64             `json:"queuedAt"`
	StartedAt     int64             `json:"startedAt"`
	UpdatedAt     int64             `json:"updatedAt"`
	Version       int               `json:"version"`
}

type ExecutionSummary struct {
	ExecutionId  int64           `json:"executionId"`
	Priority     uint8           `json:"priority"`
	Operation    string          `json:"operation"`
	RetryAttempt uint8           `json:"retryAttempt"`
	Status       ExecutionStatus `json:"status"`
	QueuedAt     int64           `json:"queuedAt"`
	StartedAt    int64           `json:"startedAt"`
	UpdatedAt    int64           `json:"updatedAt"`
}

type ExecutionSummaryForJob struct {
	ThingId          string           `json:"thingId"`
	ExecutionSummary ExecutionSummary `json:"executionSummary"`
}

type ExecutionSummaryForThing struct {
	JobId            string           `json:"jobId"`
	ExecutionSummary ExecutionSummary `json:"executionSummary"`
}

type CreateParameters struct {
	JobId                 string   `json:"jobId"`                 // optional
	Targets               []string `json:"targets"`               // thingId list
	Document              string   `json:"document"`              // job doc
	Description           string   `json:"description"`           // optional
	Priority              uint8    `json:"priority"`              // optional
	Operation             string   `json:"operation"`             // optional
	SchedulingConfig      any      `json:"schedulingConfig"`      // optional
	ExecutionsRetryConfig any      `json:"executionsRetryConfig"` // optional
	TimeoutConfig         any      `json:"timeoutConfig"`         // optional
}

type UpdateParameters struct {
	Description           string `json:"description"`           // optional
	ExecutionsRetryConfig any    `json:"executionsRetryConfig"` // optional
	TimeoutConfig         any    `json:"timeoutConfig"`         // optional
}

type CancelParameters struct {
	Comment    string `json:"comment"`
	ReasonCode string `json:"reasonCode"`
}

type CancelExecParameters struct {
	ExpectedVersion int               `json:"expectedVersion"`
	StatusDetails   map[string]string `json:"statusDetails"`
}
