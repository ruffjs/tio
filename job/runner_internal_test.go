package job

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	dbmock "ruff.io/tio/db/mock"
)

func TestUpdateTaskStatusSetsCompletedAt(t *testing.T) {
	ctx := context.Background()
	db := dbmock.NewSqliteConnTest()
	require.NoError(t, db.AutoMigrate(&TaskEntity{}))
	require.NoError(t, db.Create(&TaskEntity{
		TaskId: 1, JobId: "j1", ThingId: "t1", Operation: "op", Status: TaskSent,
	}).Error)

	r := &runnerImpl{ctx: ctx, repo: NewRepo(db)}
	r.updateTaskStatus(TaskChangeMsg{
		Task:   Task{TaskId: 1, JobId: "j1"},
		Status: TaskFailed,
	})

	te, err := r.repo.GetTask(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, TaskFailed, te.Status)
	require.NotNil(t, te.CompletedAt, "terminal status must record completed_at")
}
