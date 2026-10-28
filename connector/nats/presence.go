package nats

import (
	"context"
	"encoding/json"
	"errors"
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

	c.presenceMu.Lock()
	defer c.presenceMu.Unlock()
	c.syncLocalPresence(thingId, "")
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
	c.presenceMu.Lock()
	defer c.presenceMu.Unlock()
	c.syncLocalPresence(thingId, evt.Reason)
}

// Call with presenceMu held; each retry reads both the latest owner and connections.
func (c *Connector) syncLocalPresence(thingId, reason string) {
	for attempt := 0; attempt < 3; attempt++ {
		if c.ctx.Err() != nil {
			return
		}
		err := c.trySyncLocalPresence(thingId, reason)
		if err == nil {
			delete(c.pendingPresence, thingId)
			return
		}
		if !errors.Is(err, nats.ErrKeyExists) {
			slog.Error("update local presence", "thingId", thingId, "error", err)
			return
		}
	}
	if c.pendingPresence == nil {
		c.pendingPresence = make(map[string]string)
	}
	c.pendingPresence[thingId] = reason
	slog.Warn("presence revision conflicts; deferred reconciliation", "thingId", thingId)
}

func (c *Connector) trySyncLocalPresence(thingId, reason string) error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	key := presenceKeyPrefix + thingId
	entry, err := c.kv.Get(key)
	if err != nil && err != nats.ErrKeyNotFound {
		return fmt.Errorf("get presence record: %w", err)
	}
	var rec PresenceRecord
	var revision uint64
	if err == nil {
		if err := json.Unmarshal(entry.Value(), &rec); err != nil {
			return fmt.Errorf("parse presence record: %w", err)
		}
		revision = entry.Revision()
	}
	conns, err := c.localConnections(thingId)
	if err != nil {
		return fmt.Errorf("get local presence connections: %w", err)
	}
	connected := len(conns) > 0
	if !connected && (revision == 0 || !rec.Connected || rec.ServerId != c.cfg.Server.ServerName) {
		return nil
	}
	if connected {
		ci := conns[len(conns)-1]
		if rec.Connected && rec.ServerId == c.cfg.Server.ServerName && rec.ClientId == ci.MQTTClient && rec.RemoteAddr == ci.IP {
			return nil
		}
		rec.ClientId, rec.RemoteAddr = ci.MQTTClient, ci.IP
	}
	now := time.Now()
	rec.ThingId, rec.ServerId = thingId, c.cfg.Server.ServerName
	rec.Connected, rec.Timestamp = connected, now.UnixMilli()
	data, _ := json.Marshal(rec)
	if revision == 0 {
		_, err = c.kv.Create(key, data)
	} else {
		_, err = c.kv.Update(key, data, revision)
	}
	if err != nil {
		return err
	}
	eventType := connector.EventDisconnected
	ci := connector.ClientInfo{ClientId: thingId, Username: thingId, Connected: connected, RemoteAddr: rec.RemoteAddr}
	if connected {
		eventType = connector.EventConnected
		ci.ConnectedAt = &now
	} else {
		ci.DisconnectedAt, ci.DisconnectReason = &now, reason
	}
	c.publishMqttPresence(thingId, now, eventType, rec)
	if c.presenceHandler != nil {
		c.presenceHandler(ci)
	}
	return nil
}

func (c *Connector) localConnections(user string) ([]*server.ConnInfo, error) {
	opts := server.ConnzOptions{Username: true, Account: AppAccountName, User: user, Limit: server.DefaultConnListSize}
	var result []*server.ConnInfo
	for {
		if err := c.ctx.Err(); err != nil {
			return nil, err
		}
		page, err := c.natsSvr.Server().Connz(&opts)
		if err != nil {
			return nil, err
		}
		result = append(result, page.Conns...)
		opts.Offset += opts.Limit
		if len(page.Conns) < opts.Limit || opts.Offset >= page.Total {
			return result, nil
		}
	}
}

func (c *Connector) compensatePresence() {
	c.presenceMu.Lock()
	defer c.presenceMu.Unlock()
	conns, err := c.localConnections("")
	if err != nil {
		slog.Error("scan presence connections", "error", err)
		return
	}
	keys, err := c.kv.Keys()
	if err != nil && err != nats.ErrNoKeysFound {
		slog.Error("scan presence records", "error", err)
		return
	}
	users := make(map[string]bool)
	for _, ci := range conns {
		users[ci.AuthorizedUser] = true
	}
	for _, key := range keys {
		if strings.HasPrefix(key, presenceKeyPrefix) {
			users[strings.TrimPrefix(key, presenceKeyPrefix)] = true
		}
	}
	for user := range users {
		if user != "" && !strings.HasPrefix(user, "$") {
			c.syncLocalPresence(user, "reconciled after system reconnect")
		}
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

	if err := c.publishMqttLocked(connector.TopicPresenceEvent(thingId), 1, false, payload); err != nil {
		slog.Error("publish presence event via MQTT", "thingId", thingId, "error", err)
	}
	if err := c.publishMqttLocked(connector.TopicPresence(thingId), 1, true, payload); err != nil {
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
		case <-c.presenceReconnect:
			c.mu.Lock()
			if c.ctx.Err() == nil {
				c.compensatePresence()
			}
			c.mu.Unlock()
		case <-ticker.C:
			c.mu.Lock()
			if c.ctx.Err() == nil {
				c.reconcile()
			}
			c.mu.Unlock()
		}
	}
}

func (c *Connector) reconcile() {
	c.presenceMu.Lock()
	defer c.presenceMu.Unlock()
	if c.ctx.Err() != nil {
		return
	}
	if c.natsSvr == nil || c.kv == nil {
		return
	}

	for thingId, reason := range c.pendingPresence {
		c.syncLocalPresence(thingId, reason)
	}

	aliveServers := c.queryAliveServers()

	conns, err := c.localConnections("")
	if err != nil {
		slog.Error("reconcile: get connz", "error", err)
		return
	}

	activeUsers := make(map[string]bool)
	for _, ci := range conns {
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

		if rec.ServerId == c.cfg.Server.ServerName {
			if rec.Connected && !activeUsers[rec.ThingId] {
				c.syncLocalPresence(rec.ThingId, "reconciled stale presence")
			}
			continue
		}

		shouldDisconnect := false
		if rec.Connected && !aliveServers[rec.ServerId] {
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
