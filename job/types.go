package job

// Types for Job and Execution

// The following data types are used to communicate with the tio Jobs service over the MQTT
// Their names start with T which means thing.

type ExecStatus string

const (
	ExecQueued     ExecStatus = "QUEUED"
	ExecInProgress ExecStatus = "IN_PROGRESS"
	ExecFailed     ExecStatus = "FAILED"
	ExecSucceeded  ExecStatus = "SUCCEEDED"
	ExecCanceled   ExecStatus = "CANCELED"
	ExecTimeOut    ExecStatus = "TIMED_OUT"
	ExecRejected   ExecStatus = "REJECTED"
	ExecRemoved    ExecStatus = "REMOVED"
)

type TExec struct {
	JobId       string `json:"jobId"`
	ThingId     string `json:"thingId"`
	ExecId      int64  `json:"execId"`
	JobDocument string `json:"jobDocument"`
	Priority    uint8  `json:"priority"` // 1-10
	Operation   string `json:"operation"`

	Status        ExecStatus        `json:"status"`
	StatusDetails map[string]string `json:"statusDetails"`
	QueuedAt      int64             `json:"queuedAt"` // timestamp in ms
	StartedAt     int64             `json:"startedAt"`
	UpdatedAt     int64             `json:"updatedAt"`

	Version int `json:"version"`
}

type TExecState struct {
	Status        ExecStatus        `json:"status"`
	StatusDetails map[string]string `json:"statusDetails"`
	Version       int               `json:"version"`
}

type TExecSummary struct {
	JobId     string `json:"jobId"`
	ExecId    int64  `json:"execId"`
	Priority  uint8  `json:"priority"` // 1-10
	Operation string `json:"operation"`

	QueuedAt  int64 `json:"queuedAt"` // timestamp in ms
	StartedAt int64 `json:"startedAt"`
	UpdatedAt int64 `json:"updatedAt"`

	Version int `json:"version"`
}

// The following data types are used by management and control applications to communicate with to Jobs.

type Status string

const (
	StatusInProgress Status = "IN_PROGRESS"
	StatusCanceled   Status = "CANCELED"
	StatusSucceeded  Status = "SUCCEEDED"
	StatusScheduled  Status = "SCHEDULED"
)

type MaintenanceWindow struct {
	StartTime         string `json:"startTime"` //  cron, eg: "cron(0 0 18 ? * MON *)" means "every monday at 18:00"
	DurationInMinutes int    `json:"durationInMinutes"`
}
type SchedulingConfig struct {
	StartTime          string              `json:"startTime"`
	EndTime            string              `json:"endTime"`
	EndBehavior        string              `json:"endBehavior"`
	MaintenanceWindows []MaintenanceWindow `json:"maintenanceWindows"`
}

type RetryConfig struct {
	CriteriaList []RetryConfigItem
}
type RetryConfigItem struct {
	FailureType     string `json:"failureType"` // FAILED | TIMED_OUT | ALL
	NumberOfRetries int    `json:"numberOfRetries"`
}

type TimeoutConfig struct {
	InProgressMinutes int64 `json:"inProgressMinutes"` // max time for executions stay in "IN_PROGRESS" status
}

type ProcessDetails struct {
	ProcessingTargets []string // The target things to which the job execution is being rolled out

	// Status statistics with Thing as the statistical unit

	CountOfCanceled   int `json:"countOfCanceled"`
	CountOfFailed     int `json:"countOfFailed"`
	CountOfInProgress int `json:"countOfInProgress"`
	CountOfQueued     int `json:"countOfQueued"`
	CountOfRejected   int `json:"countOfRejected"`
	CountOfRemoved    int `json:"countOfRemoved"`
	CountOfSucceeded  int `json:"countOfSucceeded"`
	CountOfTimedOut   int `json:"countOfTimedOut"`
}

type Detail struct {
	JobId string `json:"jobId"`

	Targets          []string         `json:"targets"`
	Document         string           `json:"document"`
	Description      string           `json:"description"`
	Priority         uint8            `json:"priority"`
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
	Priority  uint8  `json:"priority"`
	Operation string `json:"operation"`

	Status      Status `json:"status"`
	StartedAt   int64  `json:"startedAt"`
	CompletedAt int64  `json:"completedAt"`
	UpdatedAt   int64  `json:"updatedAt"`

	Version int `json:"version"`
}

type Exec struct {
	JobId     string `json:"jobId"`
	ExecId    int64  `json:"execId"`
	Priority  uint8  `json:"priority"`
	Operation string `json:"operation"`

	ForceCanceled bool              `json:"forceCanceled"`
	Status        ExecStatus        `json:"status"`
	StatusDetails map[string]string `json:"statusDetails"`
	QueuedAt      int64             `json:"queuedAt"`
	StartedAt     int64             `json:"startedAt"`
	UpdatedAt     int64             `json:"updatedAt"`

	Version int `json:"version"`
}

type ExecSummary struct {
	ExecId    int64  `json:"execId"`
	Priority  uint8  `json:"priority"`
	Operation string `json:"operation"`

	RetryAttempt uint8      `json:"retryAttempt"`
	Status       ExecStatus `json:"status"`
	QueuedAt     int64      `json:"queuedAt"`
	StartedAt    int64      `json:"startedAt"`
	UpdatedAt    int64      `json:"updatedAt"`
}

type ExecSummaryForJob struct {
	ThingId     string      `json:"thingId"`
	ExecSummary ExecSummary `json:"execSummary"`
}

type ExecSummaryForThing struct {
	JobId       string      `json:"jobId"`
	ExecSummary ExecSummary `json:"execSummary"`
}

// The following is used by http api request body

type CreateParameters struct {
	JobId            string   `json:"jobId"`            // optional
	Targets          []string `json:"targets"`          // thingId list , eg ["thing/test", "thing/demo"]
	Document         string   `json:"document"`         // job doc
	Description      string   `json:"description"`      // optional
	Priority         uint8    `json:"priority"`         // optional
	Operation        string   `json:"operation"`        // optional
	SchedulingConfig any      `json:"schedulingConfig"` // optional
	RetryConfig      any      `json:"retryConfig"`      // optional, executions retry config
	TimeoutConfig    any      `json:"timeoutConfig"`    // optional
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

type CancelExecParameters struct {
	Version       int               `json:"version"`       // optional, expected version
	StatusDetails map[string]string `json:"statusDetails"` // optional
}
