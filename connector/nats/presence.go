package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/eventbus"

	server "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	presenceKVBucket     = "TIO_PRESENCE"
	presenceKeyPrefix    = "presence."
	presenceEventBusKey  = "presence"
	reconcileInterval    = 10 * time.Second
	sysConnectSubj       = "$SYS.ACCOUNT." + AppAccountName + ".CONNECT"
	sysDisconnectSubj    = "$SYS.ACCOUNT." + AppAccountName + ".DISCONNECT"
)

type PresenceRecord struct {
	ThingId    string `json:"thingId"`
	Connected  bool   `json:"connected"`
	Generation int64  `json:"generation"`
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
	c.presenceBus = eventbus.NewEventBus[connector.PresenceEvent]()

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

func (c *Connector) handleConnectEvent(msg *nats.Msg) {
	var evt sysConnectEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		slog.Error("parse connect event", "error", err)
		return
	}

	thingId := evt.Client.User
	if thingId == "" || strings.HasPrefix(thingId, "$") {
		return
	}

	serverId := evt.Server.Name
	if serverId == "" {
		serverId = evt.Server.ID
	}

	key := presenceKeyPrefix + thingId
	var gen int64
	if existing, err := c.kv.Get(key); err == nil {
		var rec PresenceRecord
		if json.Unmarshal(existing.Value(), &rec) == nil {
			gen = rec.Generation
		}
	}
	gen++

	now := time.Now()
	rec := PresenceRecord{
		ThingId:    thingId,
		Connected:  true,
		Generation: gen,
		ServerId:   serverId,
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
		slog.Error("put presence record", "key", key, "error", err)
		return
	}

	presenceEvt := connector.PresenceEvent{
		Timestamp:  now.UnixMilli(),
		EventType:  connector.EventConnected,
		ThingId:    thingId,
		ClientId:   evt.Client.MQTTClient,
		RemoteAddr: evt.Client.Host,
	}

	c.publishPresence(thingId, presenceEvt)
}

func (c *Connector) handleDisconnectEvent(msg *nats.Msg) {
	var evt sysDisconnectEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		slog.Error("parse disconnect event", "error", err)
		return
	}

	thingId := evt.Client.User
	if thingId == "" || strings.HasPrefix(thingId, "$") {
		return
	}

	key := presenceKeyPrefix + thingId
	existing, err := c.kv.Get(key)
	if err != nil {
		return
	}

	var rec PresenceRecord
	if json.Unmarshal(existing.Value(), &rec) != nil {
		return
	}

	if !rec.Connected {
		return
	}

	now := time.Now()
	rec.Connected = false
	rec.Timestamp = now.UnixMilli()

	data, err := json.Marshal(rec)
	if err != nil {
		slog.Error("marshal presence record", "error", err)
		return
	}
	if _, err := c.kv.Put(key, data); err != nil {
		slog.Error("put presence record on disconnect", "key", key, "error", err)
		return
	}

	presenceEvt := connector.PresenceEvent{
		Timestamp:        now.UnixMilli(),
		EventType:        connector.EventDisconnected,
		ThingId:          thingId,
		ClientId:         evt.Client.MQTTClient,
		RemoteAddr:       evt.Client.Host,
		DisconnectReason: evt.Reason,
	}

	c.publishPresence(thingId, presenceEvt)
}

func (c *Connector) publishPresence(thingId string, evt connector.PresenceEvent) {
	c.presenceBus.Publish(presenceEventBusKey, evt)

	payload, err := json.Marshal(evt)
	if err != nil {
		slog.Error("marshal presence event for MQTT", "error", err)
		return
	}

	if c.mqttPub != nil {
		if err := c.mqttPub.Publish(connector.TopicPresenceEvent(thingId), 1, false, payload); err != nil {
			slog.Error("publish presence event via MQTT", "thingId", thingId, "error", err)
		}
		if err := c.mqttPub.Publish(connector.TopicPresence(thingId), 1, true, payload); err != nil {
			slog.Error("publish retained presence via MQTT", "thingId", thingId, "error", err)
		}
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

	kvConnected := make(map[string]bool)
	if err != nats.ErrNoKeysFound {
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
			if rec.Connected {
				kvConnected[rec.ThingId] = true
			}
		}
	}

	for thingId := range kvConnected {
		if !activeUsers[thingId] {
			key := presenceKeyPrefix + thingId
			existing, err := c.kv.Get(key)
			if err != nil {
				continue
			}
			var rec PresenceRecord
			if json.Unmarshal(existing.Value(), &rec) != nil {
				continue
			}
			if !rec.Connected {
				continue
			}
			rec.Connected = false
			rec.Timestamp = time.Now().UnixMilli()
			data, _ := json.Marshal(rec)
			_, _ = c.kv.Put(key, data)
		}
	}
}
