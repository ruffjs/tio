package thing_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	tioconn "ruff.io/tio/connector"
	dbmock "ruff.io/tio/db/mock"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/pkg/uuid"
	"ruff.io/tio/shadow"
	"ruff.io/tio/thing"
)

func TestDeviceProfilesFromThingFields(t *testing.T) {
	db := dbmock.NewSqliteConnTest()
	require.NoError(t, db.AutoMigrate(&thing.Entity{}, &shadow.Entity{}, &shadow.ConnStatusEntity{}))
	service := shadow.NewTestSvc(shadow.NewShadowRepo(db), nil, shadow.Config{})
	ts := thing.NewSvc(thing.NewThingRepo(db), uuid.New(), service, nil)
	_, err := ts.Create(t.Context(), thing.Thing{Id: "profile-storage", Enabled: true, MessageProtocol: "simple", PayloadEncoding: "cbor"}, nil, false)
	require.NoError(t, err)
	profiles := thing.DeviceProfiles{Service: ts, DefaultCodec: "json"}
	profile, err := profiles.GetDeviceProfile(t.Context(), "profile-storage")
	require.NoError(t, err)
	require.Equal(t, tioconn.DeviceProfile{MessageProtocol: "simple", PayloadEncoding: "cbor"}, profile)

	// upsert omitting the fields keeps the stored profile
	_, err = ts.Create(t.Context(), thing.Thing{Id: "profile-storage", Enabled: true}, nil, true)
	require.NoError(t, err)
	profile, err = profiles.GetDeviceProfile(t.Context(), "profile-storage")
	require.NoError(t, err)
	require.Equal(t, tioconn.DeviceProfile{MessageProtocol: "simple", PayloadEncoding: "cbor"}, profile)

	// update invalidates the cache so the new profile is visible immediately
	mp, pe := "legacy", "json"
	require.NoError(t, ts.Update(t.Context(), "profile-storage", thing.ThingPatch{MessageProtocol: &mp, PayloadEncoding: &pe}))
	profile, err = profiles.GetDeviceProfile(t.Context(), "profile-storage")
	require.NoError(t, err)
	require.Equal(t, tioconn.DeviceProfile{MessageProtocol: "legacy", PayloadEncoding: "json"}, profile)

	_, err = ts.Create(t.Context(), thing.Thing{Id: "profile-invalid", MessageProtocol: "unknown"}, nil, false)
	var he model.HttpErr
	require.ErrorAs(t, err, &he)
	require.Equal(t, 400, he.HttpCode)

	// corrupt a stored row that was never read: the cache must not hide it
	_, err = ts.Create(t.Context(), thing.Thing{Id: "profile-corrupt", Enabled: true}, nil, false)
	require.NoError(t, err)
	require.NoError(t, db.Model(&thing.Entity{}).Where("id = ?", "profile-corrupt").Update("message_protocol", "invalid").Error)
	_, err = profiles.GetDeviceProfile(t.Context(), "profile-corrupt")
	require.ErrorAs(t, err, &he)
	require.Equal(t, 409, he.HttpCode)

	// DB failure for an uncached thing
	_, err = ts.Create(t.Context(), thing.Thing{Id: "profile-dbdown", Enabled: true}, nil, false)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = profiles.GetDeviceProfile(t.Context(), "profile-dbdown")
	require.ErrorAs(t, err, &he)
	require.Equal(t, 503, he.HttpCode)
}

func TestDeviceProfileEncodingInEveryMode(t *testing.T) {
	db := dbmock.NewSqliteConnTest()
	require.NoError(t, db.AutoMigrate(&thing.Entity{}, &shadow.Entity{}, &shadow.ConnStatusEntity{}))
	ss := shadow.NewTestSvc(shadow.NewShadowRepo(db), nil, shadow.Config{})
	ts := thing.NewSvc(thing.NewThingRepo(db), uuid.New(), ss, nil)
	id := "profile-all-modes"
	_, err := ts.Create(t.Context(), thing.Thing{Id: id, Enabled: true, MessageProtocol: "simple", PayloadEncoding: "cbor"}, nil, false)
	require.NoError(t, err)
	for _, mode := range []string{"legacy", "simple", "dual"} {
		p := thing.DeviceProfiles{Service: ts, DefaultCodec: "json", Mode: mode}
		got, err := p.GetDeviceProfile(t.Context(), id)
		require.NoError(t, err)
		wantProtocol := mode
		if mode == "dual" {
			wantProtocol = "simple"
		}
		require.Equal(t, tioconn.DeviceProfile{MessageProtocol: wantProtocol, PayloadEncoding: "cbor"}, got)
	}

	empty := ""
	require.NoError(t, ts.Update(t.Context(), id, thing.ThingPatch{MessageProtocol: &empty, PayloadEncoding: &empty}))
	for _, mode := range []string{"legacy", "simple", "dual"} {
		for _, encoding := range []string{"json", "cbor"} {
			p := thing.DeviceProfiles{Service: ts, DefaultCodec: encoding, Mode: mode}
			got, err := p.GetDeviceProfile(t.Context(), id)
			require.NoError(t, err)
			wantProtocol := mode
			if mode == "dual" {
				wantProtocol = "legacy"
			}
			require.Equal(t, tioconn.DeviceProfile{MessageProtocol: wantProtocol, PayloadEncoding: encoding}, got,
				"cached fields must use this deployment's defaults")
		}
	}
}
