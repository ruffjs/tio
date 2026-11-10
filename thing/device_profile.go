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
	Mode         string
}

func (p DeviceProfiles) GetDeviceProfile(ctx context.Context, thingId string) (connector.DeviceProfile, error) {
	var profile connector.DeviceProfile
	if v, ok := deviceProfileCache.Get(thingId); ok {
		profile = v.(connector.DeviceProfile)
	} else {
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
		profile = connector.DeviceProfile{MessageProtocol: th.MessageProtocol, PayloadEncoding: th.PayloadEncoding}
		// Cache stored fields so deployment defaults are resolved by each caller.
		deviceProfileCache.Set(thingId, profile, cache.DefaultExpiration)
	}
	if p.Mode == "legacy" || p.Mode == "simple" {
		profile.MessageProtocol = p.Mode
	} else if profile.MessageProtocol == "" {
		profile.MessageProtocol = "legacy"
	}
	if profile.PayloadEncoding == "" {
		profile.PayloadEncoding = p.DefaultCodec
	}
	return profile, nil
}

func invalidateDeviceProfile(thingId string) {
	deviceProfileCache.Delete(thingId)
}
