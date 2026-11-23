package nats

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"ruff.io/tio/connector"

	server "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type controlDisconnectRequest struct {
	ThingId     string `json:"thingId"`
	ServerId    string `json:"serverId"`
	AllowAbsent bool   `json:"allowAbsent,omitzero"`
}

type controlDisconnectResponse struct {
	Disconnected bool   `json:"disconnected"`
	Error        string `json:"error,omitzero"`
}

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

func (c *Connector) disconnectLocal(thingId string) (bool, error) {
	if c.natsSvr == nil {
		return false, server.ErrServerNotRunning
	}
	conns, err := c.localConnections(thingId)
	if err != nil {
		return false, fmt.Errorf("get connections for %q: %w", thingId, err)
	}
	var closed bool
	var disconnectErr error
	for _, ci := range conns {
		if ci.AuthorizedUser == thingId {
			if err := c.natsSvr.Server().DisconnectClientByID(ci.Cid); err == nil {
				closed = true
			} else {
				disconnectErr = errors.Join(disconnectErr, fmt.Errorf("disconnect connection %d: %w", ci.Cid, err))
			}
		}
	}
	return closed, disconnectErr
}

func (c *Connector) Close(thingId string) error {
	closed, err := c.disconnectLocal(thingId)
	if err != nil {
		return err
	}
	if closed {
		return nil
	}

	entry, err := c.kv.Get(presenceKeyPrefix + thingId)
	if err == nats.ErrKeyNotFound {
		return fmt.Errorf("%w for %q", connector.ErrConnectionNotFound, thingId)
	}
	if err != nil {
		return fmt.Errorf("get presence for %q: %w", thingId, err)
	}
	var rec PresenceRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return fmt.Errorf("unmarshal presence for %q: %w", thingId, err)
	}
	if !rec.Connected {
		return fmt.Errorf("%w for %q", connector.ErrConnectionNotFound, thingId)
	}

	return c.requestDisconnect(controlDisconnectRequest{ThingId: thingId, ServerId: rec.ServerId})
}

// CloseAll closes the thing's connections on this node and all routed peers.
func (c *Connector) CloseAll(thingId string) error {
	if c.natsSvr == nil {
		return server.ErrServerNotRunning
	}
	topology, err := c.natsSvr.Server().Routez(nil)
	if err != nil {
		return fmt.Errorf("list cluster routes: %w", err)
	}
	peers := make(map[string]struct{})
	for _, route := range topology.Routes {
		peers[route.RemoteName] = struct{}{}
	}

	results := make(chan error, len(peers))
	for peer := range peers {
		go func() {
			results <- c.requestDisconnect(controlDisconnectRequest{
				ThingId: thingId, ServerId: peer, AllowAbsent: true,
			})
		}()
	}
	_, err = c.disconnectLocal(thingId)
	for range peers {
		err = errors.Join(err, <-results)
	}
	return err
}

func (c *Connector) requestDisconnect(req controlDisconnectRequest) error {
	payload, err := json.Marshal(req, json.Deterministic(true), json.FormatNilMapAsNull(true), json.FormatNilSliceAsNull(true))
	if err != nil {
		return fmt.Errorf("marshal control disconnect: %w", err)
	}
	reply, err := c.natsConn.Request(controlDisconnectSubj, payload, systemRequestTimeout)
	if err != nil {
		return fmt.Errorf("request disconnect of %q from server %q: %w", req.ThingId, req.ServerId, err)
	}
	var resp controlDisconnectResponse
	if err := json.Unmarshal(reply.Data, &resp); err != nil {
		return fmt.Errorf("unmarshal control disconnect response: %w", err)
	}
	if !resp.Disconnected {
		if resp.Error == "" {
			resp.Error = "disconnect was not confirmed"
		}
		return fmt.Errorf("disconnect %q from server %q: %s", req.ThingId, req.ServerId, resp.Error)
	}
	return nil
}

func (c *Connector) Remove(thingId string) error {
	c.presenceMu.Lock()
	defer c.presenceMu.Unlock()
	_ = c.Close(thingId)
	delete(c.pendingPresence, thingId)

	key := presenceKeyPrefix + thingId
	if c.kv != nil {
		_ = c.kv.Delete(key)
	}

	_ = c.publishMqttLocked(connector.TopicPresence(thingId), 1, true, nil)

	return nil
}
