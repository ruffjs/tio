package thing

import (
	"context"
	"errors"
	"time"

	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/cache"
	"ruff.io/tio/pkg/model"
)

// Short TTL keeps per-message profile reads off the DB; local updates invalidate immediately.
const deviceProfileCacheTTL = 2 * time.Second

var deviceProfileCache = cache.New(deviceProfileCacheTTL, time.Minute)

type DeviceProfiles struct {
	Service      Service
	DefaultCodec string
}

func (p DeviceProfiles) GetDeviceProfile(ctx context.Context, thingId string) (connector.DeviceProfile, error) {
	if v, ok := deviceProfileCache.Get(thingId); ok {
		return v.(connector.DeviceProfile), nil
	}
	th, err := p.Service.Get(ctx, thingId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return connector.DeviceProfile{}, err
		}
		return connector.DeviceProfile{}, model.MkHttpErr("device configuration unavailable", 503, 503)
	}
	if err := model.ValidateDeviceProfile(th.MessageProtocol, th.PayloadEncoding); err != nil {
		return connector.DeviceProfile{}, model.MkHttpErr("invalid device communication configuration", 409, 409)
	}
	profile := connector.DeviceProfile{MessageProtocol: th.MessageProtocol, PayloadEncoding: th.PayloadEncoding}
	if profile.MessageProtocol == "" {
		profile.MessageProtocol = "legacy"
	}
	if profile.PayloadEncoding == "" {
		profile.PayloadEncoding = p.DefaultCodec
	}
	deviceProfileCache.Set(thingId, profile, cache.DefaultExpiration)
	return profile, nil
}

func invalidateDeviceProfile(thingId string) {
	deviceProfileCache.Delete(thingId)
}
