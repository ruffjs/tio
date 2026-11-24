package shadow

import (
	"log/slog"
	"sync"
)

// Shadow cache

type Cache interface {
	Get(thingId string) (ShadowWithStatus, bool)
	GetOrLoad(thingId string, load func() (ShadowWithStatus, bool)) (ShadowWithStatus, bool)
	// full update shadow
	Set(thingId string, shadow ShadowWithStatus)

	// partial update shadow base
	SetShadow(thingId string, shadow Shadow)
	// partial update shadow tags
	UpdateTags(thingId string, tags TagsValue)

	updateThing(thingId string, enable bool)

	Del(thingId string)
}

func newCache() Cache {
	return &cacheImpl{entries: make(map[string]*cacheEntry)}
}

type cacheImpl struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	value *ShadowWithStatus
}

func (c *cacheImpl) Set(thingId string, shadow ShadowWithStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.set(thingId, shadow)
}

func (c *cacheImpl) set(thingId string, shadow ShadowWithStatus) {
	slog.Debug("Shadow cache Set", "thingId", thingId, "shadowVersion", shadow.Version)
	c.entries[thingId] = &cacheEntry{value: &shadow}
}

func (c *cacheImpl) SetShadow(thingId string, shadow Shadow) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.get(thingId); ok {
		s.Shadow = shadow
		c.set(thingId, s)
		slog.Debug("Shadow cache SetShadow", "thingId", thingId, "shadowVersion", shadow.Version)
	} else {
		c.set(thingId, ShadowWithStatus{Shadow: shadow, Enabled: true})
	}
}

func (c *cacheImpl) Get(thingId string) (ShadowWithStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.get(thingId)
}

func (c *cacheImpl) get(thingId string) (ShadowWithStatus, bool) {
	if entry := c.entries[thingId]; entry != nil && entry.value != nil {
		return *entry.value, true
	}
	return ShadowWithStatus{}, false
}

func (c *cacheImpl) GetOrLoad(thingId string, load func() (ShadowWithStatus, bool)) (ShadowWithStatus, bool) {
	c.mu.Lock()
	if value, ok := c.get(thingId); ok {
		c.mu.Unlock()
		return value, true
	}
	entry := c.entries[thingId]
	if entry == nil {
		entry = &cacheEntry{}
		c.entries[thingId] = entry
	}
	c.mu.Unlock()

	value, ok := load()
	c.mu.Lock()
	defer c.mu.Unlock()
	// Invalidation removes the placeholder for this device.
	if c.entries[thingId] == entry {
		if ok {
			c.set(thingId, value)
		} else {
			delete(c.entries, thingId)
		}
	}
	return value, ok
}

func (c *cacheImpl) UpdateTags(thingId string, tags TagsValue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.get(thingId); ok {
		s.Tags = tags
		c.set(thingId, s)
		slog.Debug("Shadow cache UpdateTags", "thingId", thingId, "tags", tags)
	} else {
		delete(c.entries, thingId)
	}
}

func (c *cacheImpl) updateThing(thingId string, enable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.get(thingId); ok {
		s.Enabled = enable
		c.set(thingId, s)
		slog.Debug("Shadow cache updateThing", "thingId", thingId, "enable", enable)
	} else {
		delete(c.entries, thingId)
	}
}

func (c *cacheImpl) Del(thingId string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, thingId)
}
