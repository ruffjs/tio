package job

import "context"

// Center Scheduler manage the jobs and tasks according to configs, change the status of job
// Send task to Runner and Runner handle the task Queue, do the action for task
// When task status change, Runner notify it to Center to handle the status and schedule of job
//                             ┌──────────────────────────┐
//                             │         job center       │
//                             │                          │
//                             │   ┌───────────────────┐  │
//     job/task manage ───────►│   │      schedule     │  │
//                             │   │                   │  │
//    load job from db ───────►│   │       timing      │  ├──────► database(save state)
//                             │   │   retry, timeout  │  │
//                             │   └───────────────────┘  │
//                             │           ▼  ▲           │
//                             │   ┌───────────────────┐  │
//                             │   │    state trans    │  │
//                             │   └───────────────────┘  │
//                             │           ▼  ▲           │
//                             │   ┌───────────────────┐  │
//                             │   │     task queue    │  │
//  Device(get/report) ───────►│   └───────────────────┘  ├──────► Action(notify-thing/direct-method/update-shadow)
//                             │                          │
//                             └──────────────────────────┘

const centerWorkerPoolSize = 100
const runnerWorkerPoolSize = 500

// Center Entrance to task operation management
//   - Receive management messages from the upper layer
//   - Schedule job and task (according to schedule config , timeout config ...)
//   - Manage job and task status
//   - Send task to Runner
type Center interface {
	// Start context is for lifecycle of the Center, when context is canceled then Center exit
	Start(ctx context.Context) error

	// ReceiveMgrMsg Receive management message from MgrService, and return immediately.
	ReceiveMgrMsg(msg MgrMsg)

	GetPendingJobs() []PendingJobItem

	GetPendingTasks(jobId string) []Task
}

// Runner
// - Receive task from Center
// - Manage TaskQueue for thing
// - Do task operation (eg: invoke direct method, set shadow, notify task to thing)
// - Corresponding thing's task request (get / update)
// - Notify the Center of the task status change
type Runner interface {
	// Start context is for lifecycle of the Runner, when context is canceled then Runner exit
	Start(ctx context.Context, jobCtxGetter ctxGetter)

	// Receive task from center

	PutTasks(operation string, l []Task)
	DeleteTaskOfJob(jobId, operation string, force bool)
	CancelTaskOfJob(jobId, operation string, force bool)
	DeleteTask(taskId int64, operation string, force bool)
	CancelTask(taskId int64, operation string, force bool)

	// OnTaskChange Notify task change to Center
	OnTaskChange() <-chan TaskChangeMsg

	// GetPendingTasksOfSys get pending tasks for system operations
	// op: SysOpDirectMethod, SysOpUpdateShadow
	GetPendingTasksOfSys(op string) []Task
	// GetPendingTasksOfCustom get pending tasks for custom operations
	GetPendingTasksOfCustom() []Task
}

type MgrMsgType string

const (
	MgrTypeCreateJob  MgrMsgType = "createJob"
	MgrTypeUpdateJob  MgrMsgType = "updateJob"
	MgrTypeCancelJob  MgrMsgType = "cancelJob"
	MgrTypeDeleteJob  MgrMsgType = "deleteJob"
	MgrTypeCancelTask MgrMsgType = "cancelTask"
	MgrTypeDeleteTask MgrMsgType = "deleteTask"
)

type MgrMsg struct {
	Typ  MgrMsgType
	Data any
}

type MgrMsgCreateJob struct {
	JobContext   JobContext
	TargetConfig TargetConfig
}
type MgrMsgUpdateJob struct {
	JobId         string
	RetryConfig   RetryConfig
	TimeoutConfig TimeoutConfig
}
type MgrMsgCancelJob struct {
	JobId     string
	Operation string
	Force     bool
}
type MgrMsgDeleteJob struct {
	JobId     string
	Operation string
	Force     bool
}
type MgrMsgCancelTask struct {
	JobId     string
	TaskId    int64
	Operation string
	Force     bool
}
type MgrMsgDeleteTask struct {
	JobId     string
	TaskId    int64
	Operation string
	Force     bool
}

type ctxGetter func(jobId string) *JobContext
type JobContext struct {
	JobId string

	JobDoc    map[string]any
	Operation string

	SchedulingConfig *SchedulingConfig
	RolloutConfig    *RolloutConfig
	RetryConfig      *RetryConfig
	TimeoutConfig    *TimeoutConfig

	Status        Status
	ForceCanceled bool
	StartedAt     *int64
}

type TaskChangeMsg struct {
	Task Task // original task info

	// following is the new information of the task
	Status        TaskStatus
	StatusDetails StatusDetails
	Progress      uint8
	Err           error // error when do action for job
}
