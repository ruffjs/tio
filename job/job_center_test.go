package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbMock "ruff.io/tio/db/mock"
	"ruff.io/tio/job"
	"ruff.io/tio/job/test"
	"ruff.io/tio/shadow"
)

func Test_jobCenter_DirectMethodInvoke(t *testing.T) {
	ctx := context.Background()
	db := dbMock.NewSqliteConnTest()

	mkMethod := test.NewMethodHandler()

	jc := job.NewCenter(ctx, job.CenterOptions{CheckJobStatusInterval: time.Millisecond * 10},
		job.NewRepo(db), nil, &mkMethod, nil)
	svc, _ := test.NewTestSvcWithDB(db, jc)
	jc.Start(ctx)

	tests := []struct {
		name     string
		jobId    string
		respCode []int
		ok       int
		fail     int
	}{
		{
			name:     "all success",
			jobId:    "all-success",
			respCode: []int{200, 200, 200, 200},
			ok:       4,
		},
		{
			name:     "all failed",
			jobId:    "all-failed",
			respCode: []int{500, 400, 800, 720},
			fail:     4,
		},
		{
			name:     "failed part",
			jobId:    "random",
			respCode: []int{200, 530, 700, 200},
			ok:       2,
			fail:     2,
		},
	}

	for _, tt := range tests {
		st := tt
		t.Run(st.name, func(t *testing.T) {
			jd := job.InvokeDirectMethodReq{
				Method:      "testMethod",
				Data:        "hi",
				RespTimeout: 10,
			}
			createReq := job.CreateReq{
				JobId:     st.jobId,
				Operation: job.SysOpDirectMethod,
				JobDoc:    jd,
				TargetConfig: job.TargetConfig{
					Type:   job.TargetTypeThingId,
					Things: []string{"th1", "th2", "th3", "th4"},
				},
			}

			c := 0
			returnFunc := func() (shadow.MethodResp, error) {
				r := shadow.MethodResp{
					Code:    st.respCode[c],
					Message: "OK",
					Data:    "from-mock-response",
				}
				c++
				return r, nil
			}
			mCall := mkMethod.On("InvokeMethod", ctx, mock.Anything).Return(shadow.MethodResp{}, nil)
			mkMethod.SetReturnFunc(returnFunc)

			svc.CreateJob(ctx, createReq)

			time.Sleep(time.Millisecond * 100)

			mCall.Parent.AssertCalled(t, "InvokeMethod", ctx, mock.Anything)
			j, err := svc.GetJob(ctx, createReq.JobId)
			require.NoError(t, err)
			require.Equal(t, st.ok, j.ProcessDetails.Succeeded)
			require.Equal(t, st.fail, j.ProcessDetails.Failed)
			mCall.Unset()
		})
	}

}
