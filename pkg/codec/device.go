package codec

import (
	"context"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/model"
)

var ErrProtocolMismatch = model.MkHttpErr("device protocol does not match topic", 409, 409)

// ForDevice selects one codec snapshot for a device operation.
func ForDevice(ctx context.Context, fallback Codec, profiles connector.DeviceProfileGetter, thingId, protocol string) (Codec, error) {
	if c := Snapshot(ctx); c != nil {
		return c, nil
	}
	if profiles == nil {
		return fallback, nil
	}
	p, err := profiles.GetDeviceProfile(ctx, thingId)
	if err != nil {
		return nil, err
	}
	if p.MessageProtocol != protocol {
		return nil, ErrProtocolMismatch
	}
	return New(p.PayloadEncoding)
}

type snapshotKey struct{}

func WithSnapshot(ctx context.Context, c Codec) context.Context {
	return context.WithValue(ctx, snapshotKey{}, c)
}

func Snapshot(ctx context.Context) Codec {
	c, _ := ctx.Value(snapshotKey{}).(Codec)
	return c
}
