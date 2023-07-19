package job_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/db/mock"
	"ruff.io/tio/job"
	"ruff.io/tio/job/wire"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
	"testing"
	"time"
)

func NewTestSvc() job.MgrService {
	db := mock.NewSqliteConnTest()
	err := db.AutoMigrate(job.Entity{}, job.TaskEntity{})
	if err != nil {
		log.Fatalf("job auto migrate error: %v", err)
	}

	s := wire.InitSvc(db)
	return s
}

func Test_mgrSvcImpl_CreateJob(t *testing.T) {
	ctx := context.Background()
	svc := NewTestSvc()

	sdt := time.Now().Add(time.Hour * 24)
	tests := []struct {
		name    string
		req     job.CreateReq
		wantErr bool
	}{
		{
			name: "create with jobId",
			req: job.CreateReq{
				Operation: "test",
				JobId:     "test-job-001", TargetConfig: job.TargetConfig{
					Type:   "THING_ID",
					Things: []string{"a"},
				}},
			wantErr: false,
		},
		{
			name: "create with duplicated jobId should error",
			req: job.CreateReq{
				Operation: "test",
				JobId:     "test-job-001", TargetConfig: job.TargetConfig{
					Type:   "THING_ID",
					Things: []string{"b"},
				}},
			wantErr: true,
		},
		{
			name: "create without jobId",
			req: job.CreateReq{
				Operation: "test",
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
					StartTime:   time.Now(),
					EndTime:     &sdt,
					EndBehavior: job.ScheduleEndBehaviorCancel,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		subT := tt
		t.Run(subT.name, func(t *testing.T) {
			got, err := svc.CreateJob(ctx, subT.req)
			require.True(t, (err != nil) == subT.wantErr, "wantErr=%v error=%v", subT.wantErr, err)
			if subT.name == "create without jobId" {
				require.True(t, got.JobId != "", "job id is generated")
			} else if err == nil {
				if subT.req.JobId != "" {
					require.Equal(t, subT.req.JobId, got.JobId, "create job id")
				}
				require.Equal(t, subT.req.TargetConfig, got.TargetConfig)
				require.Equal(t, subT.req.Operation, got.Operation)
				require.Equal(t, job.StatusScheduled, got.Status)
			}
		})
	}
}

func Test_mgrSvcImpl_QueryJob(t *testing.T) {
	ctx := context.Background()
	svc := NewTestSvc()
	jobsTest := []job.CreateReq{
		{
			JobId:     "test1",
			Operation: "test1",
			TargetConfig: job.TargetConfig{
				Type:   "THING_ID",
				Things: []string{"a"},
			},
		},
		{
			JobId:     "test2",
			Operation: "test2",
			TargetConfig: job.TargetConfig{
				Type:   "THING_ID",
				Things: []string{"a", "b"},
			},
		},
		{
			JobId:     "test3",
			Operation: "test3",
			TargetConfig: job.TargetConfig{
				Type:   "THING_ID",
				Things: []string{"a", "b", "c"},
			},
		},
	}
	for _, j := range jobsTest {
		_, err := svc.CreateJob(ctx, j)
		require.NoError(t, err)
	}

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
			query:   job.PageQuery{PageQuery: model.PageQuery{PageIndex: 1, PageSize: 3}, Status: job.StatusScheduled},
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
			require.Equal(t, len(jobsTest), int(got.Total))
			require.Equal(t, subT.wantLen, len(got.Content))
		})
	}
}
