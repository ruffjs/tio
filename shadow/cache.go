package shadow

import (
	"log/slog"
	"sync"

	"ruff.io/tio/connector"
)

// Shadow cache

type Cache interface {
	Get(thingId string) (ShadowWithStatus, bool)
	// full update shadow
	Set(thingId string, shadow ShadowWithStatus)

	// partial update shadow base
	SetShadow(thingId string, shadow Shadow)
	// partial update shadow conn status
	UpdateConnStatus(thingId string, conn connector.ClientInfo)
	// partial update shadow tags
	UpdateTags(thingId string, tags TagsValue)

	updateThing(thingId string, enable bool)

	Del(thingId string)
}

func newCache() Cache {
	return &cacheImpl{}
}

type cacheImpl struct {
	cache sync.Map
}

func (c *cacheImpl) Set(thingId string, shadow ShadowWithStatus) {
	slog.Debug("Shadow cache Set", "thingId", thingId, "shadowVersion", shadow.Version)
	c.cache.Store(thingId, shadow)
}

func (c *cacheImpl) SetShadow(thingId string, shadow Shadow) {
	if s, ok := c.Get(thingId); ok {
		s.Shadow = shadow
		c.Set(thingId, s)
		slog.Debug("Shadow cache SetShadow", "thingId", thingId, "shadowVersion", shadow.Version)
	} else {
		c.Set(thingId, ShadowWithStatus{Shadow: shadow, Enabled: true})
	}
}

func (c *cacheImpl) Get(thingId string) (ShadowWithStatus, bool) {
	if s, ok := c.cache.Load(thingId); ok {
		sd := s.(ShadowWithStatus)
		return sd, ok
	} else {
		return ShadowWithStatus{}, false
	}
}

func (c *cacheImpl) UpdateTags(thingId string, tags TagsValue) {
	if s, ok := c.Get(thingId); ok {
		s.Tags = tags
		c.Set(thingId, s)
		slog.Debug("Shadow cache UpdateTags", "thingId", thingId, "tags", tags)
	}
}

func (c *cacheImpl) UpdateConnStatus(thingId string, conn connector.ClientInfo) {
	if s, ok := c.Get(thingId); ok {
		s.Connected = &conn.Connected
		if conn.ConnectedAt != nil {
			s.ConnectedAt = conn.ConnectedAt
		}
		if conn.DisconnectedAt != nil {
			s.DisconnectedAt = conn.DisconnectedAt
		}
		s.RemoteAddr = conn.RemoteAddr
		c.Set(thingId, s)
		slog.Debug("Shadow cache UpdateConnStatus", "thingId", thingId, "conn", conn.Connected, "shadowVersion", s.Version)
	}
}

func (c *cacheImpl) updateThing(thingId string, enable bool) {
	if s, ok := c.Get(thingId); ok {
		s.Enabled = enable
		c.Set(thingId, s)
		slog.Debug("Shadow cache updateThing", "thingId", thingId, "enable", enable)
	}
}

func (c *cacheImpl) Del(thingId string) {
	c.cache.Delete(thingId)
}
