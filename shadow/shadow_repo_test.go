package shadow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/db/mock"
	"ruff.io/tio/pkg/model"
)

// A write must never be unconditional: version 0 means "not specified by the
// client", not "overwrite whatever is there".
func TestShadowRepoUpdateRequiresVersion(t *testing.T) {
	ctx := context.Background()
	db := mock.NewSqliteConnTest()
	require.NoError(t, db.AutoMigrate(&Entity{}, &ConnStatusEntity{}))
	r := NewShadowRepo(db)

	s, err := r.Create(ctx, "thing-update-version", DefaultShadow("thing-update-version"))
	require.NoError(t, err)
	require.Equal(t, int64(1), s.Version)

	s.State.Desired["color"] = "red"
	s.Version = 2
	_, err = r.Update(ctx, "thing-update-version", 0, *s)
	require.ErrorIs(t, err, model.ErrVersionConflict)

	_, err = r.Update(ctx, "thing-update-version", 99, *s)
	require.ErrorIs(t, err, model.ErrVersionConflict)

	updated, err := r.Update(ctx, "thing-update-version", 1, *s)
	require.NoError(t, err)
	require.NotZero(t, updated.UpdatedAt)
}
