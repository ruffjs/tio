package nats

import (
	"bytes"
	"context"
	"encoding/json/v2"
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
	InstanceId string `json:"instanceId,omitempty"`
	ConnId     uint64 `json:"connId,omitempty"`
}

type sysClientInfo struct {
	User       string `json:"user,omitzero"`
	Name       string `json:"name,omitzero"`
	Host       string `json:"host,omitzero"`
	ID         uint64 `json:"id,omitzero"`
	Account    string `json:"acc,omitzero"`
	Server     string `json:"server,omitzero"`
	MQTTClient string `json:"client_id,omitzero"`
	Kind       string `json:"kind,omitzero"`
	ClientType string `json:"client_type,omitzero"`
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

		closed, err := c.disconnectLocal(req.ThingId)
		resp := controlDisconnectResponse{Disconnected: err == nil && (closed || req.AllowAbsent)}
		if err != nil {
			resp.Error = err.Error()
		} else if !resp.Disconnected {
			resp.Error = fmt.Sprintf("no connection found for %q on server %q", req.ThingId, req.ServerId)
		}
		data, err := json.Marshal(resp, json.Deterministic(true), json.FormatNilMapAsNull(true), json.FormatNilSliceAsNull(true))
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
	for range 3 {
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
	epoch, err := c.presenceStreamEpoch()
	if err != nil {
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
		instanceId := c.natsSvr.Server().ID()
		if rec.Connected && rec.InstanceId == instanceId && rec.ConnId == ci.Cid {
			return nil
		}
		rec.ClientId, rec.RemoteAddr = ci.MQTTClient, ci.IP
		rec.InstanceId, rec.ConnId = instanceId, ci.Cid
	}
	now := time.Now()
	rec.ThingId, rec.ServerId = thingId, c.cfg.Server.ServerName
	rec.Connected, rec.Timestamp = connected, now.UnixMilli()
	data, _ := json.Marshal(rec, json.Deterministic(true), json.FormatNilMapAsNull(true), json.FormatNilSliceAsNull(true))
	revision, epoch, err = c.writePresence(key, data, revision, epoch)
	if err != nil {
		return err
	}
	eventType := connector.EventDisconnected
	ci := presenceClientInfo(rec, revision, epoch)
	ci.ClientId = thingId
	if connected {
		eventType = connector.EventConnected
	} else {
		ci.DisconnectReason = reason
	}
	c.publishMqttPresence(thingId, now, eventType, rec)
	if c.presenceHandler != nil {
		c.presenceHandler(ci)
	}
	return nil
}

func (c *Connector) presenceStreamEpoch() (int64, error) {
	stream, err := c.js.StreamInfo("KV_" + presenceKVBucket)
	if err != nil {
		return 0, fmt.Errorf("get presence stream: %w", err)
	}
	return stream.Created.UnixNano(), nil
}

func (c *Connector) writePresence(key string, data []byte, revision uint64, epoch int64) (uint64, int64, error) {
	var err error
	if revision == 0 {
		revision, err = c.kv.Create(key, data)
	} else {
		revision, err = c.kv.Update(key, data, revision)
	}
	if err != nil {
		return 0, 0, err
	}
	currentEpoch, err := c.presenceStreamEpoch()
	if err != nil {
		return 0, 0, err
	}
	if currentEpoch != epoch {
		// A replacement stream can reuse revisions from the previous stream.
		entry, err := c.kv.GetRevision(key, revision)
		if errors.Is(err, nats.ErrKeyNotFound) || err == nil && !bytes.Equal(entry.Value(), data) {
			return 0, 0, nats.ErrKeyExists
		}
		if err != nil {
			return 0, 0, err
		}
	}
	return revision, currentEpoch, nil
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

func (c *Connector) compensatePresence(includeStored bool) {
	c.presenceMu.Lock()
	defer c.presenceMu.Unlock()
	conns, err := c.localConnections("")
	if err != nil {
		slog.Error("scan presence connections", "error", err)
		return
	}
	users := make(map[string]bool)
	for _, ci := range conns {
		users[ci.AuthorizedUser] = true
	}
	if includeStored {
		keys, err := c.kv.Keys()
		if err != nil && err != nats.ErrNoKeysFound {
			slog.Error("scan presence records", "error", err)
			return
		}
		for _, key := range keys {
			if after, ok := strings.CutPrefix(key, presenceKeyPrefix); ok {
				users[after] = true
			}
		}
	}
	for user := range users {
		if user != "" && !strings.HasPrefix(user, "$") {
			c.syncLocalPresence(user, "reconciled after system reconnect")
		}
	}
}

func (c *Connector) publishMqttPresence(thingId string, ts time.Time, eventType string, rec PresenceRecord) {
	if c.deviceCodec == nil {
		return
	}

	payload, err := c.deviceCodec.Marshal(connector.PresenceEvent{
		Timestamp:   ts.UnixMilli(),
		TimestampUs: ts.UnixMicro(),
		EventType:   eventType,
		ThingId:     thingId,
		ClientId:    rec.ClientId,
		RemoteAddr:  rec.RemoteAddr,
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
			if c.ctx.Err() == nil {
				c.compensatePresence(true)
			}
		case <-ticker.C:
			if c.ctx.Err() == nil {
				c.reconcile()
			}
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

	epoch, err := c.presenceStreamEpoch()
	if err != nil {
		slog.Error("reconcile: get presence stream", "error", err)
		return
	}
	keys, err := c.kv.Keys()
	if err != nil && err != nats.ErrNoKeysFound {
		slog.Error("reconcile: get KV keys", "error", err)
		return
	}

	for _, key := range keys {
		user, ok := strings.CutPrefix(key, presenceKeyPrefix)
		if !ok {
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
		active := activeUsers[user]
		delete(activeUsers, user)

		if rec.ServerId == c.cfg.Server.ServerName {
			if rec.Connected && !active {
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
			data, _ := json.Marshal(rec, json.Deterministic(true), json.FormatNilMapAsNull(true), json.FormatNilSliceAsNull(true))
			revision, epoch, err := c.writePresence(key, data, entry.Revision(), epoch)
			if errors.Is(err, nats.ErrKeyExists) {
				continue
			}
			if err != nil {
				slog.Error("reconcile: put stale presence", "key", key, "error", err)
				continue
			}

			c.publishMqttPresence(rec.ThingId, now, connector.EventDisconnected, rec)

			if c.presenceHandler != nil {
				ci := presenceClientInfo(rec, revision, epoch)
				ci.ClientId = rec.ThingId
				ci.DisconnectReason = "reconciled stale presence"
				c.presenceHandler(ci)
			}
		}
	}
	for user := range activeUsers {
		c.syncLocalPresence(user, "reconciled missing presence")
	}
}

// isSingleNode uses static config, not the momentary route count: a seed node
// may have no routes while still being part of a cluster.
func (c *Connector) isSingleNode() bool {
	return len(c.cfg.Server.Routes) == 0 && c.cfg.Server.ClusterPort == 0
}

func (c *Connector) queryAliveServers() map[string]bool {
	result := make(map[string]bool)

	if c.natsSvr != nil {
		v, err := c.natsSvr.Server().Varz(nil)
		if err == nil && v != nil {
			result[v.Name] = true
		}
	}

	// A single node only knows itself: asking $SYS only adds latency.
	if c.sysConn != nil && !c.isSingleNode() {
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

		var stats struct {
			Server struct {
				Name string `json:"name"`
			} `json:"server"`
		}
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
