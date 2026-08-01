package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ruff.io/tio/connector"

	server "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	presenceKVBucket      = "TIO_PRESENCE"
	presenceKeyPrefix     = "presence."
	reconcileInterval     = 10 * time.Second
	systemRequestTimeout  = 2 * time.Second
	systemResponseQuiet   = 200 * time.Millisecond
	sysConnectSubj        = "$SYS.ACCOUNT." + AppAccountName + ".CONNECT"
	sysDisconnectSubj     = "$SYS.ACCOUNT." + AppAccountName + ".DISCONNECT"
	controlDisconnectSubj = "$tio.control.disconnect"
)

type PresenceRecord struct {
	ThingId    string `json:"thingId"`
	Connected  bool   `json:"connected"`
	ServerId   string `json:"serverId"`
	ClientId   string `json:"clientId"`
	RemoteAddr string `json:"remoteAddr"`
	Timestamp  int64  `json:"timestamp"`
}

type sysClientInfo struct {
	User       string `json:"user,omitempty"`
	Name       string `json:"name,omitempty"`
	Host       string `json:"host,omitempty"`
	ID         uint64 `json:"id,omitempty"`
	Account    string `json:"acc,omitempty"`
	Server     string `json:"server,omitempty"`
	MQTTClient string `json:"client_id,omitempty"`
	Kind       string `json:"kind,omitempty"`
	ClientType string `json:"client_type,omitempty"`
}

type sysServerInfo struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type sysConnectEvent struct {
	Server sysServerInfo `json:"server"`
	Client sysClientInfo `json:"client"`
}

type sysDisconnectEvent struct {
	Server sysServerInfo `json:"server"`
	Client sysClientInfo `json:"client"`
	Reason string        `json:"reason"`
}

func (c *Connector) initPresence() error {
	kv, err := c.js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:   presenceKVBucket,
		History:  5,
		Replicas: c.cfg.Server.PresenceReplicas,
	})
	if err != nil {
		return fmt.Errorf("create presence KV bucket: %w", err)
	}
	c.kv = kv

	if _, err := c.sysConn.Subscribe(sysConnectSubj, c.handleConnectEvent); err != nil {
		return fmt.Errorf("subscribe connect events: %w", err)
	}
	if _, err := c.sysConn.Subscribe(sysDisconnectSubj, c.handleDisconnectEvent); err != nil {
		return fmt.Errorf("subscribe disconnect events: %w", err)
	}
	if err := c.sysConn.Flush(); err != nil {
		return fmt.Errorf("flush presence subscriptions: %w", err)
	}

	go c.startReconciliation(c.ctx)

	return nil
}

func (c *Connector) initControl() error {
	_, err := c.natsConn.Subscribe(controlDisconnectSubj, func(msg *nats.Msg) {
		var req controlDisconnectRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}
		// In a cluster, all servers receive this message, but only the owner should respond.
		if req.ServerId != c.cfg.Server.ServerName {
			return
		}

		resp := controlDisconnectResponse{Disconnected: c.disconnectLocal(req.ThingId)}
		if !resp.Disconnected {
			resp.Error = fmt.Sprintf("no connection found for %q on server %q", req.ThingId, req.ServerId)
		}
		data, err := json.Marshal(resp)
		if err != nil {
			return
		}
		if err := msg.Respond(data); err != nil {
			slog.Error("control disconnect: respond", "thingId", req.ThingId, "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("subscribe control disconnect: %w", err)
	}
	return c.natsConn.Flush()
}

func (c *Connector) handleConnectEvent(msg *nats.Msg) {
	var evt sysConnectEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		slog.Error("parse connect event", "error", err)
		return
	}

	if evt.Server.Name != c.cfg.Server.ServerName {
		return
	}

	thingId := evt.Client.User
	if thingId == "" || strings.HasPrefix(thingId, "$") {
		return
	}

	now := time.Now()
	rec := PresenceRecord{
		ThingId:    thingId,
		Connected:  true,
		ServerId:   evt.Server.Name,
		ClientId:   evt.Client.MQTTClient,
		RemoteAddr: evt.Client.Host,
		Timestamp:  now.UnixMilli(),
	}

	data, err := json.Marshal(rec)
	if err != nil {
		slog.Error("marshal presence record", "error", err)
		return
	}
	key := presenceKeyPrefix + thingId
	if _, err := c.kv.Put(key, data); err != nil {
		slog.Error("put presence record", "key", key, "error", err)
		return
	}

	c.publishMqttPresence(thingId, now, connector.EventConnected, rec)

	if c.presenceHandler != nil {
		c.presenceHandler(connector.ClientInfo{
			ClientId:    thingId,
			Username:    thingId,
			Connected:   true,
			ConnectedAt: &now,
			RemoteAddr:  evt.Client.Host,
		})
	}
}

func (c *Connector) handleDisconnectEvent(msg *nats.Msg) {
	var evt sysDisconnectEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		slog.Error("parse disconnect event", "error", err)
		return
	}

	if evt.Server.Name != c.cfg.Server.ServerName {
		return
	}

	thingId := evt.Client.User
	if thingId == "" || strings.HasPrefix(thingId, "$") {
		return
	}

	key := presenceKeyPrefix + thingId

	// If the key was already removed (e.g. via Remove), don't recreate it.
	if _, err := c.kv.Get(key); err == nats.ErrKeyNotFound {
		return
	}

	now := time.Now()
	rec := PresenceRecord{
		ThingId:    thingId,
		Connected:  false,
		ServerId:   evt.Server.Name,
		ClientId:   evt.Client.MQTTClient,
		RemoteAddr: evt.Client.Host,
		Timestamp:  now.UnixMilli(),
	}

	data, err := json.Marshal(rec)
	if err != nil {
		slog.Error("marshal presence record", "error", err)
		return
	}
	if _, err := c.kv.Put(key, data); err != nil {
		slog.Error("put presence record on disconnect", "key", key, "error", err)
		return
	}

	c.publishMqttPresence(thingId, now, connector.EventDisconnected, rec)

	if c.presenceHandler != nil {
		c.presenceHandler(connector.ClientInfo{
			ClientId:         thingId,
			Username:         thingId,
			Connected:        false,
			DisconnectedAt:   &now,
			DisconnectReason: evt.Reason,
			RemoteAddr:       evt.Client.Host,
		})
	}
}

func (c *Connector) publishMqttPresence(thingId string, ts time.Time, eventType string, rec PresenceRecord) {
	if c.protocolMode == "simple" {
		return
	}

	if c.deviceCodec == nil {
		return
	}

	payload, err := c.deviceCodec.Marshal(connector.PresenceEvent{
		Timestamp:  ts.UnixMilli(),
		EventType:  eventType,
		ThingId:    thingId,
		ClientId:   rec.ClientId,
		RemoteAddr: rec.RemoteAddr,
	})
	if err != nil {
		slog.Error("marshal presence event", "thingId", thingId, "error", err)
		return
	}

	if err := c.publishMqtt(connector.TopicPresenceEvent(thingId), 1, false, payload); err != nil {
		slog.Error("publish presence event via MQTT", "thingId", thingId, "error", err)
	}
	if err := c.publishMqtt(connector.TopicPresence(thingId), 1, true, payload); err != nil {
		slog.Error("publish retained presence via MQTT", "thingId", thingId, "error", err)
	}
}

func (c *Connector) startReconciliation(ctx context.Context) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reconcile()
		}
	}
}

func (c *Connector) reconcile() {
	if c.natsSvr == nil || c.kv == nil {
		return
	}

	aliveServers := c.queryAliveServers()

	opts := server.ConnzOptions{Username: true}
	connz, err := c.natsSvr.Server().Connz(&opts)
	if err != nil {
		slog.Error("reconcile: get connz", "error", err)
		return
	}

	activeUsers := make(map[string]bool)
	for _, ci := range connz.Conns {
		user := ci.AuthorizedUser
		if user == "" || strings.HasPrefix(user, "$") {
			continue
		}
		activeUsers[user] = true
	}

	keys, err := c.kv.Keys()
	if err != nil && err != nats.ErrNoKeysFound {
		slog.Error("reconcile: get KV keys", "error", err)
		return
	}

	for _, key := range keys {
		if !strings.HasPrefix(key, presenceKeyPrefix) {
			continue
		}
		entry, err := c.kv.Get(key)
		if err != nil {
			continue
		}
		var rec PresenceRecord
		if json.Unmarshal(entry.Value(), &rec) != nil {
			continue
		}

		shouldDisconnect := false
		if rec.Connected && !aliveServers[rec.ServerId] {
			shouldDisconnect = true
		}
		if rec.Connected && rec.ServerId == c.cfg.Server.ServerName && !activeUsers[rec.ThingId] {
			shouldDisconnect = true
		}

		if shouldDisconnect {
			now := time.Now()
			rec.Connected = false
			rec.Timestamp = now.UnixMilli()
			data, _ := json.Marshal(rec)
			if _, err := c.kv.Put(key, data); err != nil {
				slog.Error("reconcile: put stale presence", "key", key, "error", err)
				continue
			}

			c.publishMqttPresence(rec.ThingId, now, connector.EventDisconnected, rec)

			if c.presenceHandler != nil {
				c.presenceHandler(connector.ClientInfo{
					ClientId:         rec.ThingId,
					Username:         rec.ThingId,
					Connected:        false,
					DisconnectedAt:   &now,
					DisconnectReason: "reconciled stale presence",
					RemoteAddr:       rec.RemoteAddr,
				})
			}
		}
	}
}

func (c *Connector) queryAliveServers() map[string]bool {
	result := make(map[string]bool)

	if c.natsSvr != nil {
		v, err := c.natsSvr.Server().Varz(nil)
		if err == nil && v != nil {
			result[v.Name] = true
		}
	}

	if c.sysConn != nil {
		for name := range c.requestAliveServers("$SYS.REQ.SERVER.PING", systemRequestTimeout) {
			result[name] = true
		}
	}

	if len(result) == 0 && c.natsSvr != nil {
		v, _ := c.natsSvr.Server().Varz(nil)
		if v != nil {
			result[v.Name] = true
		}
	}

	return result
}

func (c *Connector) requestAliveServers(subject string, timeout time.Duration) map[string]bool {
	result := make(map[string]bool)
	inbox := nats.NewInbox()
	sub, err := c.sysConn.SubscribeSync(inbox)
	if err != nil {
		return result
	}
	defer sub.Unsubscribe()

	if err := c.sysConn.PublishRequest(subject, inbox, nil); err != nil {
		return result
	}
	if err := c.sysConn.Flush(); err != nil {
		return result
	}

	// First iteration waits the full timeout; subsequent iterations use the quiet period.
	// If no servers respond, the full timeout elapses before returning an empty result.
	deadline := time.Now().Add(timeout)
	wait := timeout
	for {
		msg, err := sub.NextMsg(wait)
		if err != nil {
			return result
		}

		var stats server.ServerStatsMsg
		if json.Unmarshal(msg.Data, &stats) == nil && stats.Server.Name != "" {
			result[stats.Server.Name] = true
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return result
		}
		wait = min(systemResponseQuiet, remaining)
	}
}
