package job_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/job"
	"ruff.io/tio/job/test"
	"ruff.io/tio/pkg/model"
)

func Test_mgrSvcImpl_CreateJob(t *testing.T) {
	ctx := context.Background()
	mockJc := test.NewMockJobCenter()
	svc, _ := test.NewTestSvc(mockJc)

	sdSt := time.Now().Truncate(time.Millisecond)
	sdEt := sdSt.Add(time.Hour * 24)
	tests := []struct {
		name    string
		req     job.CreateReq
		wantErr bool
	}{
		{
			name: "create with jobId",
			req: job.CreateReq{
				Operation: "test",
				JobId:     "test-job-001",
				JobDoc:    map[string]any{"hi": "you"},
				TargetConfig: job.TargetConfig{
					Type:   "THING_ID",
					Things: []string{"a"},
				}},
			wantErr: false,
		},
		{
			name: "create with duplicated jobId should error",
			req: job.CreateReq{
				Operation: "test",
				JobId:     "test-job-001",
				JobDoc:    map[string]any{"hi": true},
				TargetConfig: job.TargetConfig{
					Type:   "THING_ID",
					Things: []string{"b"},
				}},
			wantErr: true,
		},
		{
			name: "create without jobId",
			req: job.CreateReq{
				Operation: "test",
				JobDoc:    map[string]any{"hi": 332.0},
				TargetConfig: job.TargetConfig{
					Type:   "THING_ID",
					Things: []string{"c"},
				}},
			wantErr: false,
		},
		{
			name: "create without operation should error",
			req: job.CreateReq{
				TargetConfig: job.TargetConfig{
					Type:   "THING_ID",
					Things: []string{"d"},
				}},
			wantErr: true,
		},
		{
			name:    "create without target config should error",
			req:     job.CreateReq{JobId: "test-job-001", Operation: "hi"},
			wantErr: true,
		},
		{
			name: "create  with full",
			req: job.CreateReq{
				JobId:       "test-job-full",
				Operation:   "test",
				Description: "desc",
				TargetConfig: job.TargetConfig{
					Type:   "THING_ID",
					Things: []string{"e"},
				},
				SchedulingConfig: &job.SchedulingConfig{
					StartTime:   sdSt,
					EndTime:     &sdEt,
					EndBehavior: job.ScheduleEndBehaviorCancel,
				},
				RolloutConfig: &job.RolloutConfig{MaxPerMinute: 10},
			},
			wantErr: false,
		},
	}

	c := 0
	for _, tt := range tests {
		subT := tt
		mockJc.On("ReceiveMgrMsg", mock.AnythingOfType("job.MgrMsg")).Return(nil)
		t.Run(subT.name, func(t *testing.T) {
			got, err := svc.CreateJob(ctx, subT.req)
			require.True(t, (err != nil) == subT.wantErr, "wantErr=%v error=%v", subT.wantErr, err)
			if err == nil {
				c += 1
				if subT.req.JobId != "" {
					require.Equal(t, subT.req.JobId, got.JobId, "create job id")
				} else {
					require.True(t, got.JobId != "", "job id is generated")
				}
				require.Equal(t, subT.req.TargetConfig, got.TargetConfig)
				require.Equal(t, subT.req.Operation, got.Operation)
				require.Equal(t, job.StatusWaiting, got.Status)

				msg := job.MgrMsg{Typ: job.MgrTypeCreateJob, Data: job.MgrMsgCreateJob{
					TargetConfig: subT.req.TargetConfig,
					JobContext: job.JobContext{JobId: got.JobId, JobDoc: subT.req.JobDoc,
						Operation:        subT.req.Operation,
						SchedulingConfig: subT.req.SchedulingConfig,
						RolloutConfig:    subT.req.RolloutConfig,
						RetryConfig:      subT.req.RetryConfig,
						TimeoutConfig:    subT.req.TimeoutConfig,
						Status:           job.StatusWaiting,
					},
				},
				}
				require.Equal(t, c, len(mockJc.Calls))
				c := mockJc.Calls[len(mockJc.Calls)-1]
				require.Equal(t, "ReceiveMgrMsg", c.Method)
				require.Equal(t, msg, c.Arguments[0])
			}

		})
	}
}

var jobsTest = []job.CreateReq{
	{
		JobId:     "test1",
		Operation: "test1",
		JobDoc:    map[string]any{"hi": "you"},
		TargetConfig: job.TargetConfig{
			Type:   "THING_ID",
			Things: []string{"a"},
		},
	},
	{
		JobId:     "test2",
		Operation: "test2",
		JobDoc:    map[string]any{"hi": 1},
		TargetConfig: job.TargetConfig{
			Type:   "THING_ID",
			Things: []string{"a", "b"},
		},
	},
	{
		JobId:     "test3",
		Operation: "test3",
		JobDoc:    map[string]any{"hi": true},
		TargetConfig: job.TargetConfig{
			Type:   "THING_ID",
			Things: []string{"a", "b", "c"},
		},
	},
}

var tasksTest = []job.TaskEntity{
	{
		JobId:     "test1",
		ThingId:   "th1",
		TaskId:    1,
		Operation: "test1",
		Status:    job.TaskQueued,
		QueuedAt:  time.Now(),
	},
	{
		JobId:     "test2",
		ThingId:   "th2",
		TaskId:    2,
		Operation: "test2",
		Status:    job.TaskQueued,
		QueuedAt:  time.Now(),
	},
	{
		JobId:     "test2",
		ThingId:   "th2-2-running",
		TaskId:    3,
		Operation: "test3",
		Status:    job.TaskInProgress,
		QueuedAt:  time.Now(),
	},
	{
		JobId:     "test3",
		ThingId:   "th3",
		TaskId:    4,
		Operation: "test3",
		Status:    job.TaskQueued,
		QueuedAt:  time.Now(),
	},
	{
		JobId:     "test3",
		ThingId:   "th4-2-running",
		TaskId:    5,
		Operation: "test3",
		Status:    job.TaskInProgress,
		QueuedAt:  time.Now(),
	},
}

func testTasksOfJob(jobId string, status job.TaskStatus) []job.TaskEntity {
	var l []job.TaskEntity
	for _, t := range tasksTest {
		if t.JobId == jobId && (status == "" || status == t.Status) {
			l = append(l, t)
		}
	}
	return l
}

func preCreateJobs(ctx context.Context, t *testing.T, svc job.MgrService, mockJc *test.JobCenter) {
	for _, j := range jobsTest {
		mockJc.On("ReceiveMgrMsg", mock.Anything, mock.Anything).Return(nil)
		_, err := svc.CreateJob(ctx, j)
		require.NoError(t, err)
	}
}
func preCreateTasks(ctx context.Context, t *testing.T, repo job.Repo) {
	_, err := repo.CreateTasks(ctx, tasksTest)
	require.NoError(t, err)
}

func Test_mgrSvcImpl_QueryJob(t *testing.T) {
	ctx := context.Background()
	mockJc := test.NewMockJobCenter()
	svc, _ := test.NewTestSvc(mockJc)
	preCreateJobs(ctx, t, svc, mockJc)

	tests := []struct {
		name    string
		query   job.PageQuery
		wantLen int
	}{
		{
			name:    "query page 1",
			query:   job.PageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 2}},
			wantLen: 2,
		},
		{
			name:    "query page 2",
			query:   job.PageQuery{PageQuery: model.PageQuery{PageIndex: 2, PageSize: 2}},
			wantLen: 1,
		},
		{
			name:    "query page with status",
			query:   job.PageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 3}, Status: job.StatusWaiting},
			wantLen: 3,
		},
		{
			name:    "query page with status not exist",
			query:   job.PageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 3}, Status: "not-exist"},
			wantLen: 0,
		},
		{
			name:    "query page with operation",
			query:   job.PageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 3}, Operation: "test2"},
			wantLen: 1,
		},
		{
			name:    "query page with operation not exist",
			query:   job.PageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 3}, Operation: "not-exist"},
			wantLen: 0,
		},
	}
	for _, tt := range tests {
		subT := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.QueryJob(ctx, subT.query)
			require.NoError(t, err)
			//require.Equal(t, len(jobsTest), int(got.Total))
			require.Equal(t, subT.wantLen, len(got.Content))
		})
	}
}

func Test_mgrSvcImpl_UpdateJob(t *testing.T) {
	ctx := context.Background()
	mockJc := test.NewMockJobCenter()
	svc, _ := test.NewTestSvc(mockJc)
	preCreateJobs(ctx, t, svc, mockJc)

	des250 := strings.Repeat("x", 250)
	des251 := strings.Repeat("x", 251)
	genReConf := func(failureTypes []string, numRetries []int) *job.RetryConfig {
		reConf := job.RetryConfig{CriteriaList: []job.RetryConfigItem{}}
		for i, ft := range failureTypes {
			reConf.CriteriaList = append(reConf.CriteriaList,
				job.RetryConfigItem{FailureType: ft, NumberOfRetries: numRetries[i]})
		}
		return &reConf
	}
	genTmConf := func(m int) *job.TimeoutConfig {
		return &job.TimeoutConfig{InProgressMinutes: m}
	}

	tests := []struct {
		name    string
		jobId   string
		req     job.UpdateReq
		wantErr bool
	}{
		{
			name:  "with too long description",
			jobId: jobsTest[0].JobId,
			req: job.UpdateReq{
				Description: &des251,
			},
			wantErr: true,
		},
		{
			name:  "with all",
			jobId: jobsTest[1].JobId,
			req: job.UpdateReq{
				Description:   &des250,
				RetryConfig:   genReConf([]string{job.TaskFailed.String(), job.TaskTimeOut.String()}, []int{3, 4}),
				TimeoutConfig: genTmConf(60 * 24),
			},
		},
		{
			name:  "with long timeout",
			jobId: jobsTest[2].JobId,
			req: job.UpdateReq{
				Description:   &des250,
				RetryConfig:   genReConf([]string{job.TaskFailed.String(), job.TaskTimeOut.String()}, []int{3, 4}),
				TimeoutConfig: genTmConf(60 * 24 * 7),
			},
		},
		{
			name:  "with too long timeout",
			jobId: jobsTest[0].JobId,
			req: job.UpdateReq{
				Description:   &des250,
				RetryConfig:   genReConf([]string{job.TaskFailed.String(), job.TaskTimeOut.String()}, []int{3, 4}),
				TimeoutConfig: genTmConf(60*24*7 + 1),
			},
			wantErr: true,
		},
		{
			name:  "with failure type ALL",
			jobId: jobsTest[1].JobId,
			req: job.UpdateReq{
				Description:   &des250,
				RetryConfig:   genReConf([]string{"ALL"}, []int{3}),
				TimeoutConfig: genTmConf(60 * 24 * 7),
			},
		},
		{
			name:  "with too many failure types",
			jobId: jobsTest[2].JobId,
			req: job.UpdateReq{
				Description:   &des250,
				RetryConfig:   genReConf([]string{"ALL", job.TaskFailed.String()}, []int{3, 4}),
				TimeoutConfig: genTmConf(60 * 24 * 7),
			},
			wantErr: true,
		},
	}

	mockJc.On("ReceiveMgrMsg", mock.AnythingOfType("job.MgrMsg")).Return(nil)
	c := len(jobsTest)
	for _, tt := range tests {
		st := tt
		t.Run(tt.name, func(t *testing.T) {
			err := svc.UpdateJob(ctx, st.jobId, st.req)
			require.True(t, (err != nil) == st.wantErr, "wantErr=%v error=%v", st.wantErr, err)
			if err == nil {
				c++
				j, err := svc.GetJob(ctx, st.jobId)
				require.NoError(t, err)
				if st.req.Description != nil {
					require.Equal(t, *st.req.Description, j.Description)
				}
				if st.req.RetryConfig != nil {
					require.Equal(t, st.req.RetryConfig, j.RetryConfig)
				}
				if st.req.TimeoutConfig != nil {
					require.Equal(t, st.req.TimeoutConfig, j.TimeoutConfig)
				}

				msg := job.MgrMsg{
					Typ:  job.MgrTypeUpdateJob,
					Data: job.MgrMsgUpdateJob{JobId: st.jobId, RetryConfig: st.req.RetryConfig, TimeoutConfig: st.req.TimeoutConfig},
				}
				require.Equal(t, c, len(mockJc.Calls))
				c := mockJc.Calls[len(mockJc.Calls)-1]
				require.Equal(t, "ReceiveMgrMsg", c.Method)
				require.Equal(t, msg, c.Arguments[0])
			}
		})
	}
}

func Test_mgrSvcImpl_CancelJob(t *testing.T) {
	ctx := context.Background()
	mockJc := test.NewMockJobCenter()
	svc, repo := test.NewTestSvc(mockJc)
	preCreateJobs(ctx, t, svc, mockJc)
	preCreateTasks(ctx, t, repo)
	err := repo.UpdateJob(ctx, jobsTest[2].JobId, map[string]any{"status": job.StatusInProgress})
	require.NoError(t, err)

	code := "xyz"
	comment := "abc"
	tests := []struct {
		name    string
		jobId   string
		force   bool
		req     job.CancelReq
		wantErr bool
	}{
		{
			name:  "cancel witch code and comment",
			jobId: jobsTest[0].JobId,
			force: false,
			req:   job.CancelReq{ReasonCode: &code, Comment: &comment},
		},
		{
			name:  "cancel witch no info",
			jobId: jobsTest[1].JobId,
			force: false,
			req:   job.CancelReq{},
		},
		{
			name:    "cancel in progress job",
			jobId:   jobsTest[2].JobId,
			force:   false,
			req:     job.CancelReq{},
			wantErr: true,
		},
		{
			name:  "cancel in progress job force",
			jobId: jobsTest[2].JobId,
			force: true,
			req:   job.CancelReq{},
		},
	}

	c := len(jobsTest)
	mockJc.On("ReceiveMgrMsg", mock.Anything)
	for _, tt := range tests {
		st := tt
		t.Run(st.name, func(t *testing.T) {
			err := svc.CancelJob(ctx, st.jobId, st.req, st.force)
			require.True(t, (err != nil) == st.wantErr, "wantErr=%v error=%v", st.wantErr, err)
			if err == nil {
				c++
				j, err := svc.GetJob(ctx, st.jobId)
				require.NoError(t, err)
				cm := ""
				rc := ""
				if st.req.Comment != nil {
					cm = *st.req.Comment
				}
				if st.req.ReasonCode != nil {
					rc = *st.req.ReasonCode
				}
				require.Equal(t, cm, j.Comment)
				require.Equal(t, rc, j.ReasonCode)

				// check notify
				msg := job.MgrMsg{
					Typ:  job.MgrTypeCancelJob,
					Data: job.MgrMsgCancelJob{JobId: st.jobId, Operation: j.Operation, Force: st.force},
				}
				require.Equal(t, c, len(mockJc.Calls))
				c := mockJc.Calls[len(mockJc.Calls)-1]
				require.Equal(t, "ReceiveMgrMsg", c.Method)
				require.Equal(t, msg, c.Arguments[0])

				// tasks status check
				tasks := testTasksOfJob(st.jobId, "")
				for _, tk := range tasks {
					tkEn, err := repo.GetTask(ctx, tk.TaskId)
					require.NoError(t, err)
					if st.force {
						require.Equal(t, job.TaskCanceled, tkEn.Status, "all task should be canceled")
					} else {
						if tk.Status == job.TaskInProgress {
							require.Equal(t, job.TaskInProgress, tkEn.Status,
								"in progress task should not be canceled")
						} else {
							require.Equal(t, job.TaskCanceled, tkEn.Status, "task(not in progress) should be canceled")
						}
					}
				}
			}
		})
	}
}

func Test_mgrSvcImpl_DeleteJob(t *testing.T) {
	ctx := context.Background()
	mockJc := test.NewMockJobCenter()
	svc, repo := test.NewTestSvc(mockJc)
	preCreateJobs(ctx, t, svc, mockJc)
	preCreateTasks(ctx, t, repo)
	err := repo.UpdateJob(ctx, jobsTest[1].JobId, map[string]any{"status": job.StatusInProgress})
	require.NoError(t, err)
	err = repo.UpdateJob(ctx, jobsTest[2].JobId, map[string]any{"status": job.StatusInProgress})
	require.NoError(t, err)

	tests := []struct {
		name    string
		jobId   string
		force   bool
		wantErr bool
	}{
		{
			name:  "delete job",
			jobId: jobsTest[0].JobId,
			force: false,
		},
		{
			name:  "delete in progress force",
			jobId: jobsTest[1].JobId,
			force: true,
		},
		{
			name:    "delete in progress with no force",
			jobId:   jobsTest[2].JobId,
			force:   false,
			wantErr: true,
		},
	}

	c := len(jobsTest)
	mockJc.On("ReceiveMgrMsg", mock.Anything)
	for _, tt := range tests {
		st := tt
		t.Run(tt.name, func(t *testing.T) {
			old, err := svc.DeleteJob(ctx, st.jobId, st.force)
			require.True(t, (err != nil) == st.wantErr, "wantErr=%v error=%v", st.wantErr, err)
			if err == nil {
				c++
				j, err := svc.GetJob(ctx, st.jobId)
				require.NoError(t, err)
				require.True(t, j == nil)
				l, err := svc.QueryTaskForJob(ctx, st.jobId, job.TaskPageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 1}})
				require.NoError(t, err)
				require.Equal(t, int64(0), l.Total, "tasks under job should be deleted")

				// check notify
				msg := job.MgrMsg{
					Typ:  job.MgrTypeDeleteJob,
					Data: job.MgrMsgDeleteJob{JobId: st.jobId, Operation: old.Operation, Force: st.force},
				}
				require.Equal(t, c, len(mockJc.Calls))
				c := mockJc.Calls[len(mockJc.Calls)-1]
				require.Equal(t, "ReceiveMgrMsg", c.Method)
				require.Equal(t, msg, c.Arguments[0])
			}
		})
	}
}

func Test_mgrSvcImpl_CancelTask(t *testing.T) {
	ctx := context.Background()
	mockJc := test.NewMockJobCenter()
	svc, repo := test.NewTestSvc(mockJc)
	preCreateJobs(ctx, t, svc, mockJc)
	preCreateTasks(ctx, t, repo)

	stDet := func(m map[string]any) *job.StatusDetails {
		n := job.StatusDetails(m)
		return &n
	}

	tests := []struct {
		name       string
		jobId      string
		thingId    string
		req        job.CancelTaskReq
		force      bool
		wantStatus job.TaskStatus
		wantErr    bool
	}{
		{
			name:       "cancel queued",
			jobId:      jobsTest[0].JobId,
			thingId:    testTasksOfJob(jobsTest[0].JobId, job.TaskQueued)[0].ThingId,
			req:        job.CancelTaskReq{Version: 1, StatusDetails: stDet(map[string]any{"a": 1.0, "b": "b"})},
			wantStatus: job.TaskCanceled,
		},
		{
			name:       "cancel in progress error",
			jobId:      jobsTest[1].JobId,
			thingId:    testTasksOfJob(jobsTest[1].JobId, job.TaskInProgress)[0].ThingId,
			req:        job.CancelTaskReq{Version: 1, StatusDetails: stDet(map[string]any{"a": 1.0, "b": "b"})},
			wantStatus: job.TaskInProgress,
			wantErr:    true,
		},
		{
			name:       "cancel with wrong version",
			jobId:      jobsTest[2].JobId,
			thingId:    testTasksOfJob(jobsTest[2].JobId, job.TaskQueued)[0].ThingId,
			req:        job.CancelTaskReq{Version: 12, StatusDetails: stDet(map[string]any{"a": 1.0, "b": "b"})},
			wantStatus: job.TaskQueued,
			wantErr:    true,
		},
	}

	c := len(jobsTest)
	for _, tt := range tests {
		st := tt
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CancelTask(ctx, st.thingId, st.jobId, st.req, st.force)
			require.True(t, (err != nil) == st.wantErr, "wantErr=%v error=%v", st.wantErr, err)
			if err != nil {
				return
			}
			l, err := repo.QueryTask(ctx, st.jobId, st.thingId, job.TaskPageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 1}})
			require.NoError(t, err)
			require.Equal(t, 1, len(l.Content))
			tk := l.Content[0]
			require.Equal(t, st.wantStatus, tk.Status)
			if st.wantStatus == job.TaskCanceled && st.req.StatusDetails != nil {
				var sd job.StatusDetails
				err := json.Unmarshal(tk.StatusDetails, &sd)
				require.NoError(t, err)
				require.Equal(t, *st.req.StatusDetails, sd)

				// check notify
				c++
				msg := job.MgrMsg{
					Typ:  job.MgrTypeCancelTask,
					Data: job.MgrMsgCancelTask{JobId: st.jobId, TaskId: tk.TaskId, Operation: tk.Operation, Force: st.force},
				}
				require.Equal(t, c, len(mockJc.Calls))
				c := mockJc.Calls[len(mockJc.Calls)-1]
				require.Equal(t, "ReceiveMgrMsg", c.Method)
				require.Equal(t, msg, c.Arguments[0])
			}
		})
	}
}

func Test_mgrSvcImpl_DeleteTask(t *testing.T) {
	ctx := context.Background()
	mockJc := test.NewMockJobCenter()
	svc, repo := test.NewTestSvc(mockJc)
	preCreateJobs(ctx, t, svc, mockJc)
	preCreateTasks(ctx, t, repo)

	tests := []struct {
		name    string
		jobId   string
		thingId string
		taskId  int64
		force   bool
		wantErr bool
	}{
		{
			name:    "delete queued",
			jobId:   jobsTest[0].JobId,
			thingId: testTasksOfJob(jobsTest[0].JobId, job.TaskQueued)[0].ThingId,
			taskId:  testTasksOfJob(jobsTest[0].JobId, job.TaskQueued)[0].TaskId,
		},
		{
			name:    "can't delete in progress ",
			jobId:   jobsTest[1].JobId,
			thingId: testTasksOfJob(jobsTest[1].JobId, job.TaskInProgress)[0].ThingId,
			taskId:  testTasksOfJob(jobsTest[1].JobId, job.TaskInProgress)[0].TaskId,
			wantErr: true,
		},
		{
			name:    "can delete in progress force",
			jobId:   jobsTest[1].JobId,
			thingId: testTasksOfJob(jobsTest[1].JobId, job.TaskInProgress)[0].ThingId,
			taskId:  testTasksOfJob(jobsTest[1].JobId, job.TaskInProgress)[0].TaskId,
			force:   true,
		},
	}

	c := len(jobsTest)
	mockJc.On("ReceiveMgrMsg", mock.Anything)
	for _, tt := range tests {
		st := tt
		t.Run(tt.name, func(t *testing.T) {
			old, err := svc.DeleteTask(ctx, st.thingId, st.jobId, st.taskId, st.force)
			require.True(t, (err != nil) == st.wantErr, "wantErr=%v error=%v", st.wantErr, err)
			if err != nil {
				return
			}
			tk, err := svc.GetTask(ctx, st.thingId, st.jobId, st.taskId)
			require.NoError(t, err)
			require.Nil(t, tk)

			// check notify
			c++
			msg := job.MgrMsg{
				Typ:  job.MgrTypeDeleteTask,
				Data: job.MgrMsgDeleteTask{JobId: st.jobId, TaskId: st.taskId, Operation: old.Operation, Force: st.force},
			}
			require.Equal(t, c, len(mockJc.Calls))
			c := mockJc.Calls[len(mockJc.Calls)-1]
			require.Equal(t, "ReceiveMgrMsg", c.Method)
			require.Equal(t, msg, c.Arguments[0])
		})
	}
}
