package codec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/model"
)

type stubProfiles struct {
	profile connector.DeviceProfile
	err     error
	calls   int
}

func (s *stubProfiles) GetDeviceProfile(context.Context, string) (connector.DeviceProfile, error) {
	s.calls++
	return s.profile, s.err
}

func TestForDevice(t *testing.T) {
	ctx := context.Background()
	fallback, err := New("json")
	require.NoError(t, err)
	cbor, err := New("cbor")
	require.NoError(t, err)

	t.Run("nil profiles falls back", func(t *testing.T) {
		c, err := ForDevice(ctx, fallback, nil, "thing", "legacy")
		require.NoError(t, err)
		require.Equal(t, fallback, c)
	})

	t.Run("snapshot wins without reading the profile", func(t *testing.T) {
		p := &stubProfiles{profile: connector.DeviceProfile{MessageProtocol: "legacy", PayloadEncoding: "json"}}
		c, err := ForDevice(WithSnapshot(ctx, cbor), fallback, p, "thing", "simple")
		require.NoError(t, err)
		require.IsType(t, cbor, c)
		require.Zero(t, p.calls)
	})

	t.Run("protocol mismatch", func(t *testing.T) {
		p := &stubProfiles{profile: connector.DeviceProfile{MessageProtocol: "legacy", PayloadEncoding: "json"}}
		_, err := ForDevice(ctx, fallback, p, "thing", "simple")
		require.ErrorIs(t, err, ErrProtocolMismatch)
	})

	t.Run("uses profile encoding", func(t *testing.T) {
		p := &stubProfiles{profile: connector.DeviceProfile{MessageProtocol: "legacy", PayloadEncoding: "cbor"}}
		c, err := ForDevice(ctx, fallback, p, "thing", "legacy")
		require.NoError(t, err)
		require.IsType(t, cbor, c)
	})

	t.Run("profile error propagates", func(t *testing.T) {
		p := &stubProfiles{err: model.ErrNotFound}
		_, err := ForDevice(ctx, fallback, p, "thing", "legacy")
		require.ErrorIs(t, err, model.ErrNotFound)
	})
}
