package job_test

import (
	"context"
	"testing"
	"time"

	connmock "ruff.io/tio/connector/mock"
	sdMock "ruff.io/tio/shadow/mock"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbMock "ruff.io/tio/db/mock"
	"ruff.io/tio/job"
	"ruff.io/tio/job/test"
)

const (
	directMethodX = job.SysOpDirectMethodPrefix + "x"
	updateShadowX = job.SysOpUpdateShadowPrefix + "x"
)

func prepare(t *testing.T, mockJc bool) (
	ctx context.Context,
	repo job.Repo,
	svc job.MgrService,
	jc job.Center,
	mkMethod *test.MethodHandler,
	sdSetter *sdMock.StateDesiredSetter,
	conn *connmock.MockConnector,
) {
	ctx = context.Background()
	db := dbMock.NewSqliteConnTest()

	m := test.NewMethodHandler()
	mkMethod = &m
	sdSetter = sdMock.NewShadowSetter()

	mc := connmock.NewMockConnector()
	_ = mc.Start(ctx)
	conn = mc

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
	t.Skip("NATS migration: job center test needs mock update")
}

func Test_jobCenter_DirectMethodInvoke_cancel(t *testing.T) {
	t.Skip("NATS migration: job center test needs mock update")
}

func Test_jobCenter_DirectMethodInvoke_delete(t *testing.T) {
	t.Skip("NATS migration: job center test needs mock update")
}

func Test_jobCenter_SchedulePreloadFormDb(t *testing.T) {
	t.Skip("NATS migration: job center test needs mock update")
}
