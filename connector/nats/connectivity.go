package nats

import (
	"encoding/json"
	"fmt"
	"time"

	"ruff.io/tio/connector"

	server "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func (c *Connector) IsConnected(thingId string) (bool, error) {
	entry, err := c.kv.Get(presenceKeyPrefix + thingId)
	if err == nats.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get presence for %q: %w", thingId, err)
	}
	var rec PresenceRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return false, fmt.Errorf("unmarshal presence for %q: %w", thingId, err)
	}
	return rec.Connected, nil
}

func (c *Connector) OnConnect() <-chan connector.PresenceEvent {
	return c.presenceBus.Subscribe(presenceEventBusKey)
}

func (c *Connector) ClientInfo(thingId string) (connector.ClientInfo, error) {
	entry, err := c.kv.Get(presenceKeyPrefix + thingId)
	if err == nats.ErrKeyNotFound {
		return connector.ClientInfo{}, fmt.Errorf("presence not found for %q", thingId)
	}
	if err != nil {
		return connector.ClientInfo{}, fmt.Errorf("get presence for %q: %w", thingId, err)
	}
	var rec PresenceRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return connector.ClientInfo{}, fmt.Errorf("unmarshal presence for %q: %w", thingId, err)
	}

	ci := connector.ClientInfo{
		ClientId:   rec.ClientId,
		Username:   rec.ThingId,
		Connected:  rec.Connected,
		RemoteAddr: rec.RemoteAddr,
	}
	ts := time.UnixMilli(rec.Timestamp)
	if rec.Connected {
		ci.ConnectedAt = &ts
	} else {
		ci.DisconnectedAt = &ts
	}
	return ci, nil
}

func (c *Connector) AllClientInfo() ([]connector.ClientInfo, error) {
	keys, err := c.kv.Keys()
	if err != nil && err != nats.ErrNoKeysFound {
		return nil, fmt.Errorf("list presence keys: %w", err)
	}

	var result []connector.ClientInfo
	for _, key := range keys {
		entry, err := c.kv.Get(key)
		if err != nil {
			continue
		}
		var rec PresenceRecord
		if json.Unmarshal(entry.Value(), &rec) != nil {
			continue
		}
		ci := connector.ClientInfo{
			ClientId:   rec.ClientId,
			Username:   rec.ThingId,
			Connected:  rec.Connected,
			RemoteAddr: rec.RemoteAddr,
		}
		ts := time.UnixMilli(rec.Timestamp)
		if rec.Connected {
			ci.ConnectedAt = &ts
		} else {
			ci.DisconnectedAt = &ts
		}
		result = append(result, ci)
	}
	return result, nil
}

func (c *Connector) Close(thingId string) error {
	if c.natsSvr == nil {
		return fmt.Errorf("server not available")
	}

	opts := server.ConnzOptions{
		Username: true,
		User:     thingId,
	}
	connz, err := c.natsSvr.Server().Connz(&opts)
	if err != nil {
		return fmt.Errorf("get connz for %q: %w", thingId, err)
	}

	var closed bool
	for _, ci := range connz.Conns {
		if ci.AuthorizedUser == thingId {
			if err := c.natsSvr.Server().DisconnectClientByID(ci.Cid); err != nil {
				return fmt.Errorf("disconnect client %d: %w", ci.Cid, err)
			}
			closed = true
		}
	}

	if !closed {
		return fmt.Errorf("no connection found for %q", thingId)
	}
	return nil
}

func (c *Connector) Remove(thingId string) error {
	key := presenceKeyPrefix + thingId
	if c.kv != nil {
		_ = c.kv.Purge(key)
	}

	_ = c.Close(thingId)

	if c.mqttPub != nil {
		_ = c.mqttPub.Publish(connector.TopicPresence(thingId), 1, true, nil)
	}

	return nil
}
