package shadow

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector"
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

type presenceSnapshot struct {
	connector.ConnectChecker
	clients []connector.ClientInfo
}

func (s presenceSnapshot) AllClientInfo() ([]connector.ClientInfo, error) {
	return s.clients, nil
}

func TestStartupPresenceUsesThingIDAndOrdersSameMillisecondUpdates(t *testing.T) {
	ctx := context.Background()
	db := mock.NewSqliteConnTest()
	require.NoError(t, db.AutoMigrate(&Entity{}, &ConnStatusEntity{}))
	repo := NewShadowRepo(db)
	const thingID = "device-1"
	const clientID = "mqtt-device-1"
	stamp := time.Now().Add(-time.Minute).Truncate(time.Millisecond)
	rounded := stamp.Add(time.Millisecond)
	for _, id := range []string{thingID, clientID} {
		require.NoError(t, db.Create(&ConnStatusEntity{
			ThingId: id, Connected: true, ConnectedAt: &rounded, UpdatedAt: stamp,
		}).Error)
	}
	online := connector.ClientInfo{
		ClientId: clientID, Username: thingID, Connected: true, ConnectedAt: &stamp,
		PresenceEpoch: 10, PresenceRevision: 100,
	}
	svc := NewSvc(repo, presenceSnapshot{clients: []connector.ClientInfo{online}}, Config{})
	status := func(id string) ConnStatusEntity {
		t.Helper()
		var row ConnStatusEntity
		require.NoError(t, db.Where("thing_id = ?", id).Take(&row).Error)
		return row
	}

	svc.Init(ctx)
	require.True(t, status(thingID).Connected)
	require.False(t, status(clientID).Connected)
	require.EqualValues(t, 100, status(thingID).PresenceRevision)
	require.True(t, stamp.Equal(*status(thingID).ConnectedAt))

	// A restart must keep rows represented by the same snapshot revision.
	require.NoError(t, db.Model(&ConnStatusEntity{ThingId: thingID}).
		UpdateColumn("updated_at", stamp).Error)
	svc.Init(ctx)
	require.True(t, status(thingID).Connected)

	online.ClientId = thingID
	offline := connector.ClientInfo{
		ClientId: thingID, DisconnectedAt: &stamp, PresenceEpoch: 10, PresenceRevision: 101,
		DisconnectReason: "Client Closed",
	}
	svc.HandleLocalPresence(ctx, offline)
	svc.HandleLocalPresence(ctx, online)
	require.False(t, status(thingID).Connected)
	require.EqualValues(t, 101, status(thingID).PresenceRevision)
	replay := offline
	replay.DisconnectReason = ""
	require.NoError(t, repo.UpdateConnStatus(ctx, []connector.ClientInfo{replay}, true))
	require.Equal(t, offline.DisconnectReason, status(thingID).DisconnectReason)

	// Recreating the stream starts a new sequence. Late events from its
	// predecessor still cannot overwrite the accepted update.
	online.PresenceEpoch, online.PresenceRevision = 11, 1
	svc.HandleLocalPresence(ctx, online)
	svc.HandleLocalPresence(ctx, offline)
	require.True(t, status(thingID).Connected)
	require.EqualValues(t, 1, status(thingID).PresenceRevision)
}
