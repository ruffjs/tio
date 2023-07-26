package job_test

import (
	"context"
	"encoding/json"
	"ruff.io/tio/connector"
	mqMock "ruff.io/tio/connector/mqtt/mock"
	sdMock "ruff.io/tio/shadow/mock"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbMock "ruff.io/tio/db/mock"
	"ruff.io/tio/job"
	"ruff.io/tio/job/test"
	"ruff.io/tio/shadow"
)

func prepare(t *testing.T, mockJc bool) (
	ctx context.Context,
	repo job.Repo,
	svc job.MgrService,
	jc job.Center,
	mkMethod *test.MethodHandler,
	sdSetter *sdMock.StateDesiredSetter,
	conn *mqMock.AdapterImpl,
	onConnCh chan connector.Event,
) {
	ctx = context.Background()
	db := dbMock.NewSqliteConnTest()

	m := test.NewMethodHandler()
	mkMethod = &m
	sdSetter = sdMock.NewShadowSetter()

	mockMqtt := mqMock.NewMqttClient("", nil, nil)
	c := mqMock.NewAdapter(mockMqtt)
	conn = &c
	onConnCh = make(chan connector.Event)
	var outCh <-chan connector.Event = onConnCh
	conn.On("OnConnect").Return(outCh)

	repo = job.NewRepo(db)
	if mockJc {
		mkJc := test.NewMockJobCenter()
		mkJc.On("ReceiveMgrMsg", mock.Anything)
		jc = mkJc
	} else {
		jc = job.NewCenter(
			job.CenterOptions{
				CheckJobStatusInterval: time.Millisecond * 2,
				ScheduleInterval:       time.Millisecond * 2},
			repo, nil, conn, mkMethod, sdSetter)
	}
	svc, _ = test.NewTestSvcWithDB(db, jc)
	err := jc.Start(ctx)
	require.NoError(t, err)

	return
}

func Test_jobCenter_sysOperation(t *testing.T) {
	ctx, _, svc, _, mkMethod, mkShadow, conn, onConnCh := prepare(t, false)
	tests := []struct {
		name     string
		jobId    string
		op       string
		respCode []int
		ok       int
		fail     int
		offline  bool
		reOnline bool
	}{
		{
			name:     "all success",
			jobId:    "all-success",
			op:       job.SysOpDirectMethod,
			respCode: []int{200, 200, 200, 200},
			ok:       4,
		},
		{
			name:     "all failed",
			jobId:    "all-failed",
			op:       job.SysOpDirectMethod,
			respCode: []int{500, 400, 800, 720},
			fail:     4,
		},
		{
			name:     "failed part",
			jobId:    "failed-part",
			op:       job.SysOpDirectMethod,
			respCode: []int{200, 530, 700, 200},
			ok:       2,
			fail:     2,
		},
		{
			name:     "re online",
			jobId:    "re-online",
			op:       job.SysOpDirectMethod,
			respCode: []int{200, 200, 200, 200},
			ok:       4,
			fail:     0,
			offline:  true,
			reOnline: true,
		},
		{
			name:  "shadow update",
			jobId: "shadow-update",
			op:    job.SysOpUpdateShadow,
			ok:    4,
			fail:  0,
		},
	}

	for _, tt := range tests {
		st := tt
		t.Run(st.name, func(t *testing.T) {
			isDirectMethod := st.op == job.SysOpDirectMethod
			jd := job.InvokeDirectMethodReq{
				Method:      "testMethod",
				Data:        "hi",
				RespTimeout: 10,
			}
			var m map[string]any
			b, _ := json.Marshal(jd)
			_ = json.Unmarshal(b, &m)
			createReq := job.CreateReq{
				JobId:     st.jobId,
				Operation: st.op,
				JobDoc:    m,
				TargetConfig: job.TargetConfig{
					Type:   job.TargetTypeThingId,
					Things: []string{"th1", "th2", "th3", "th4"},
				},
			}

			wg := sync.WaitGroup{}
			c := 0
			returnFunc := func() (shadow.MethodResp, error) {
				r := shadow.MethodResp{
					Code:    st.respCode[c],
					Message: "OK",
					Data:    "from-mock-response",
				}
				c++
				wg.Done()
				return r, nil
			}

			sCall := mkShadow.On("SetDesired", ctx, mock.Anything, mock.Anything).Return(shadow.Shadow{}, nil)
			mCall := mkMethod.On("InvokeMethod", ctx, mock.Anything).Return(shadow.MethodResp{}, nil)
			mkMethod.SetReturnFunc(returnFunc)

			connCall := conn.On("IsConnected", mock.Anything).Return(!st.offline, nil)

			if isDirectMethod {
				wg.Add(len(createReq.TargetConfig.Things))
			}

			_, err := svc.CreateJob(ctx, createReq)
			require.NoError(t, err)

			// wait task down
			if !st.offline {
				wg.Wait()
				// wait task state save
				time.Sleep(time.Millisecond * 60)
				if isDirectMethod {
					mCall.Parent.AssertCalled(t, "InvokeMethod", ctx, mock.Anything)
				} else {
					time.Sleep(time.Millisecond * 60)
					sCall.Parent.AssertCalled(t, "SetDesired", ctx, mock.Anything, mock.Anything)
				}
			} else if st.reOnline {
				// wait task to be handled
				time.Sleep(time.Millisecond * 60)

				j, err := svc.GetJob(ctx, createReq.JobId)
				require.NoError(t, err)
				require.Equal(t, 0, j.ProcessDetails.Succeeded)
				require.Equal(t, 4, j.ProcessDetails.Queued)

				// mock thing connect
				connCall.Unset()
				connCall = conn.On("IsConnected", mock.Anything).Return(true, nil)
				for _, th := range createReq.TargetConfig.Things {
					onConnCh <- connector.Event{ThingId: th, EventType: connector.EventConnected}
				}
				// wait task to be completed
				time.Sleep(time.Millisecond * 60)
				if isDirectMethod {
					mCall.Parent.AssertCalled(t, "InvokeMethod", ctx, mock.Anything)
				} else {
					sCall.Parent.AssertCalled(t, "SetDesired", ctx, mock.Anything, mock.Anything)
				}
			} else {
				time.Sleep(time.Second)
				//mCall.Parent.AssertNumberOfCalls(t, "InvokeMethod", 0)
			}

			j, err := svc.GetJob(ctx, createReq.JobId)
			require.NoError(t, err)
			require.Equal(t, st.ok, j.ProcessDetails.Succeeded)
			require.Equal(t, st.fail, j.ProcessDetails.Failed)

			sCall.Unset()
			mCall.Unset()
			connCall.Unset()
		})
	}

}

func Test_jobCenter_DirectMethodInvoke_cancel(t *testing.T) {
	ctx, _, svc, jc, mkMethod, _, conn, _ := prepare(t, false)
	conn.On("IsConnected", mock.Anything).Return(true, nil)

	tests := []struct {
		name              string
		jobId             string
		scheduleStartTime time.Time
		rolloutConf       *job.RolloutConfig

		force bool

		pendingJob   int
		pendingTask  int
		completeTask int
	}{
		{
			name:              "not scheduled",
			jobId:             "not-scheduled",
			scheduleStartTime: time.Now().Add(time.Second * 20),
			force:             true,
			pendingJob:        1,
			pendingTask:       4,
			completeTask:      0,
		},
		{
			name:              "not scheduled not force",
			jobId:             "not-scheduled-no-force",
			scheduleStartTime: time.Now().Add(time.Second * 20),
			force:             false,
			pendingJob:        1,
			pendingTask:       4,
			completeTask:      0,
		},
		{
			name:         "not rollout complete",
			jobId:        "not-rollout-complete",
			rolloutConf:  &job.RolloutConfig{MaxPerMinute: 1},
			force:        true,
			pendingJob:   1,
			pendingTask:  3,
			completeTask: 1,
		},
		{
			name:         "rollout complete",
			jobId:        "rollout-complete",
			force:        true,
			pendingJob:   0,
			pendingTask:  0,
			completeTask: 4,
		},
	}

	calls := 0
	for _, tt := range tests {
		st := tt
		t.Run(st.name, func(t *testing.T) {
			jd := job.InvokeDirectMethodReq{
				Method:      "testMethod",
				Data:        "hi",
				RespTimeout: 10,
			}
			var m map[string]any
			b, _ := json.Marshal(jd)
			_ = json.Unmarshal(b, &m)

			var sdConf *job.SchedulingConfig = nil
			if !st.scheduleStartTime.IsZero() {
				sdConf = &job.SchedulingConfig{
					StartTime:   st.scheduleStartTime,
					EndBehavior: job.ScheduleEndBehaviorCancel,
				}
			}

			createReq := job.CreateReq{
				JobId:     st.jobId,
				Operation: job.SysOpDirectMethod,
				JobDoc:    m,
				TargetConfig: job.TargetConfig{
					Type:   job.TargetTypeThingId,
					Things: []string{"th1", "th2", "th3", "th4"},
				},
				SchedulingConfig: sdConf,
				RolloutConfig:    st.rolloutConf,
			}

			returnFunc := func() (shadow.MethodResp, error) {
				r := shadow.MethodResp{
					Code:    200,
					Message: "OK",
					Data:    "from-mock-response",
				}
				return r, nil
			}

			mCall := mkMethod.On("InvokeMethod", ctx, mock.Anything).Return(shadow.MethodResp{}, nil)
			mkMethod.SetReturnFunc(returnFunc)

			_, err := svc.CreateJob(ctx, createReq)
			require.NoError(t, err)

			time.Sleep(time.Millisecond * 60)

			// check job is added to pending queue
			jl := jc.GetPendingJobs()
			tl := jc.GetPendingTasks(st.jobId)
			require.Equal(t, st.pendingJob, len(jl))
			if len(jl) > 0 {
				require.Equal(t, createReq.JobId, jl[0].Context.JobId)
			}
			require.Equal(t, st.pendingTask, len(tl))

			cm := "comment-" + st.jobId
			code := "code-" + st.jobId
			cReq := job.CancelReq{Comment: &cm, ReasonCode: &code}
			err = svc.CancelJob(ctx, createReq.JobId, cReq, st.force)
			require.NoError(t, err)
			j, err := svc.GetJob(ctx, st.jobId)
			require.NoError(t, err)
			require.Equal(t, *cReq.Comment, j.Comment)
			require.Equal(t, *cReq.ReasonCode, j.ReasonCode)

			time.Sleep(time.Millisecond * 80)

			jl = jc.GetPendingJobs()
			tl = jc.GetPendingTasks(st.jobId)
			if st.scheduleStartTime.After(time.Now()) {
				// check job is removed from pending queue
				require.Equal(t, 0, len(jl))
				require.Equal(t, 0, len(tl))
			} else {
				require.Equal(t, 0, len(jl))
				require.Equal(t, 0, len(tl))
			}
			calls += st.completeTask
			mCall.Parent.AssertNumberOfCalls(t, "InvokeMethod", calls)
			mCall.Unset()
		})
	}

}

func Test_jobCenter_DirectMethodInvoke_delete(t *testing.T) {
	ctx, _, svc, jc, mkMethod, _, conn, _ := prepare(t, false)
	conn.On("IsConnected", mock.Anything).Return(true, nil)

	tests := []struct {
		name              string
		jobId             string
		scheduleStartTime time.Time
		rolloutConf       *job.RolloutConfig

		force bool

		pendingJob   int
		pendingTask  int
		completeTask int
	}{
		{
			name:              "not scheduled",
			jobId:             "not-scheduled",
			scheduleStartTime: time.Now().Add(time.Second * 20),
			force:             true,
			pendingJob:        1,
			pendingTask:       4,
			completeTask:      0,
		},
		{
			name:              "not scheduled not force",
			jobId:             "not-scheduled-no-force",
			scheduleStartTime: time.Now().Add(time.Second * 20),
			force:             false,
			pendingJob:        1,
			pendingTask:       4,
			completeTask:      0,
		},
		{
			name:         "not rollout complete",
			jobId:        "not-rollout-complete",
			rolloutConf:  &job.RolloutConfig{MaxPerMinute: 1},
			force:        true,
			pendingJob:   1,
			pendingTask:  3,
			completeTask: 1,
		},
		{
			name:         "rollout complete",
			jobId:        "rollout-complete",
			force:        true,
			pendingJob:   0,
			pendingTask:  0,
			completeTask: 4,
		},
	}

	calls := 0
	for _, tt := range tests {
		st := tt
		t.Run(st.name, func(t *testing.T) {
			jd := job.InvokeDirectMethodReq{
				Method:      "testMethod",
				Data:        "hi",
				RespTimeout: 10,
			}
			var m map[string]any
			b, _ := json.Marshal(jd)
			_ = json.Unmarshal(b, &m)

			var sdConf *job.SchedulingConfig = nil
			if !st.scheduleStartTime.IsZero() {
				sdConf = &job.SchedulingConfig{
					StartTime:   st.scheduleStartTime,
					EndBehavior: job.ScheduleEndBehaviorCancel,
				}
			}

			createReq := job.CreateReq{
				JobId:     st.jobId,
				Operation: job.SysOpDirectMethod,
				JobDoc:    m,
				TargetConfig: job.TargetConfig{
					Type:   job.TargetTypeThingId,
					Things: []string{"th1", "th2", "th3", "th4"},
				},
				SchedulingConfig: sdConf,
				RolloutConfig:    st.rolloutConf,
			}

			returnFunc := func() (shadow.MethodResp, error) {
				r := shadow.MethodResp{
					Code:    200,
					Message: "OK",
					Data:    "from-mock-response",
				}
				return r, nil
			}

			mCall := mkMethod.On("InvokeMethod", ctx, mock.Anything).Return(shadow.MethodResp{}, nil)
			mkMethod.SetReturnFunc(returnFunc)

			_, err := svc.CreateJob(ctx, createReq)
			require.NoError(t, err)

			time.Sleep(time.Millisecond * 60)

			// check job is added to pending queue
			jl := jc.GetPendingJobs()
			tl := jc.GetPendingTasks(st.jobId)
			require.Equal(t, st.pendingJob, len(jl))
			if len(jl) > 0 {
				require.Equal(t, createReq.JobId, jl[0].Context.JobId)
			}
			require.Equal(t, st.pendingTask, len(tl))

			_, err = svc.DeleteJob(ctx, createReq.JobId, st.force)
			require.NoError(t, err)

			time.Sleep(time.Millisecond * 60)

			jl = jc.GetPendingJobs()
			tl = jc.GetPendingTasks(st.jobId)
			if st.scheduleStartTime.After(time.Now()) {
				// check job is removed from pending queue
				require.Equal(t, 0, len(jl))
				require.Equal(t, 0, len(tl))
			} else {
				require.Equal(t, 0, len(jl))
				require.Equal(t, 0, len(tl))
			}
			calls += st.completeTask
			mCall.Parent.AssertNumberOfCalls(t, "InvokeMethod", calls)
			mCall.Unset()
		})
	}

}

func Test_jobCenter_SchedulePreloadFormDb(t *testing.T) {
	// mock JobCenter to avoid create tasks when creat job
	ctx, repo, svc, _, mkMethod, _, conn, _ := prepare(t, true)

	// the real JobCenter for test
	jc := job.NewCenter(job.CenterOptions{
		CheckJobStatusInterval: time.Millisecond * 10,
		ScheduleInterval:       time.Millisecond * 2},
		repo, nil, conn, mkMethod, nil)
	conn.On("IsConnected", mock.Anything).Return(true, nil)
	tests := []struct {
		name              string
		jobId             string
		scheduleStartTime time.Time
		rolloutConf       *job.RolloutConfig

		things []struct {
			id     string
			status job.TaskStatus
		}

		pendingJob   int
		pendingTask  int
		completeTask int
	}{
		{
			name:  "all complete",
			jobId: "all-complete",
			things: []struct {
				id     string
				status job.TaskStatus
			}{
				{id: "th1", status: job.TaskQueued},
				{id: "th2", status: job.TaskSent},
				{id: "th3", status: job.TaskInProgress},
				{id: "th4", status: job.TaskInProgress},
			},
			pendingJob:   0,
			pendingTask:  0,
			completeTask: 4,
		},
		{
			name:  "part complete",
			jobId: "part-complete",
			things: []struct {
				id     string
				status job.TaskStatus
			}{
				{id: "th1", status: job.TaskQueued},
				{id: "th2", status: job.TaskSent},
				{id: "th3", status: job.TaskFailed},
				{id: "th4", status: job.TaskRejected},
			},
			pendingJob:   0,
			pendingTask:  0,
			completeTask: 2,
		},
		{
			name:  "empty task",
			jobId: "empty-task",
			things: []struct {
				id     string
				status job.TaskStatus
			}{
				{id: "th1", status: job.TaskCanceled},
				{id: "th2", status: job.TaskTimeOut},
				{id: "th3", status: job.TaskFailed},
				{id: "th4", status: job.TaskRejected},
			},
			pendingJob:   0,
			pendingTask:  0,
			completeTask: 0,
		},
	}

	calls := 0
	for _, tt := range tests {
		st := tt
		t.Run(st.name, func(t *testing.T) {
			jd := job.InvokeDirectMethodReq{
				Method:      "testMethod",
				Data:        "hi",
				RespTimeout: 10,
			}
			var m map[string]any
			b, _ := json.Marshal(jd)
			_ = json.Unmarshal(b, &m)

			var sdConf *job.SchedulingConfig = nil
			if !st.scheduleStartTime.IsZero() {
				sdConf = &job.SchedulingConfig{
					StartTime:   st.scheduleStartTime,
					EndBehavior: job.ScheduleEndBehaviorCancel,
				}
			}

			createReq := job.CreateReq{
				JobId:            st.jobId,
				Operation:        job.SysOpDirectMethod,
				JobDoc:           m,
				SchedulingConfig: sdConf,
				RolloutConfig:    st.rolloutConf,
				TargetConfig: job.TargetConfig{
					Type:   job.TargetTypeThingId,
					Things: []string{"th1", "th2", "th3", "th4"},
				},
			}

			returnFunc := func() (shadow.MethodResp, error) {
				r := shadow.MethodResp{
					Code:    200,
					Message: "OK",
					Data:    "from-mock-response",
				}
				return r, nil
			}

			mCall := mkMethod.On("InvokeMethod", ctx, mock.Anything).Return(shadow.MethodResp{}, nil)
			mkMethod.SetReturnFunc(returnFunc)

			// create job without invoke the real JobCenter
			_, err := svc.CreateJob(ctx, createReq)
			require.NoError(t, err)
			var el []job.TaskEntity
			for _, i := range st.things {
				el = append(el, job.TaskEntity{JobId: st.jobId, ThingId: i.id, Status: i.status,
					Operation: createReq.Operation, QueuedAt: time.Now()})
			}
			_, err = repo.CreateTasks(ctx, el)
			require.NoError(t, err)

			// start JobCenter to load job from db
			err = jc.Start(ctx)
			require.NoError(t, err)

			time.Sleep(time.Millisecond * 100)

			// check job is added to pending queue
			jl := jc.GetPendingJobs()
			tl := jc.GetPendingTasks(st.jobId)
			require.Equal(t, st.pendingJob, len(jl))
			if len(jl) > 0 {
				require.Equal(t, createReq.JobId, jl[0].Context.JobId)
			}
			require.Equal(t, st.pendingTask, len(tl))

			calls += st.completeTask
			mCall.Parent.AssertNumberOfCalls(t, "InvokeMethod", calls)
			mCall.Unset()
		})
	}

}
