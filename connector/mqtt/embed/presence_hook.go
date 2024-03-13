package embed

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ruff.io/tio/connector"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

type presenceHook struct {
	mqtt.HookBase
	publishEventFn func(topic string, retain bool, evt connector.Event)
	getClientFn    func(id string) (*mqtt.Client, bool)
}

func (h *presenceHook) ID() string {
	return "presence"
}

func (h *presenceHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnSessionEstablished,
		mqtt.OnDisconnect,
	}, []byte{b})
}

func (h *presenceHook) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	slog.Debug("Mqtt OnConnect", "clientId", cl.ID, "username", cl.Properties.Username, "ip", cl.Net.Remote)
	exist, ok := h.getClientFn(cl.ID)
	if !ok || exist.Closed() {
		slog.Debug("Ignore OnConnect message "+
			"cause client is disconnected,"+
			" may be concurrent connect and disconnect",
			"clientId", cl.ID, "username", cl.Properties.Username)
		return
	}
	now := time.Now()
	cinfo := toClientInfo(cl, true, &now, nil, nil)
	broker.updateClient(cinfo)
	if isPublishPresent(string(cl.Properties.Username)) {
		evt := toEvent(cl, connector.EventConnected, now, "")
		go func() {
			h.publishEventFn(connector.TopicPresence(cl.ID), true, evt)
			h.publishEventFn(connector.TopicPresenceEvent(cl.ID), false, evt)
		}()
	}
}

func (h *presenceHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	slog.Debug("Mqtt OnDisconnect", "clientId", cl.ID, "username", cl.Properties.Username, "ip", cl.Net.Remote)

	exist, ok := h.getClientFn(cl.ID)
	if ok && !exist.Closed() {
		slog.Debug("Ignore OnDisconnect message "+
			"cause client is connected,"+
			" may be concurrent connect and disconnect",
			"clientId", cl.ID, "username", cl.Properties.Username)
		return
	}
	now := time.Now()
	cinfo := toClientInfo(cl, false, nil, &now, err)
	broker.updateClient(cinfo)
	if isPublishPresent(string(cl.Properties.Username)) {
		evt := toEvent(cl, connector.EventDisconnected, now, fmt.Sprintf("%s", err))
		go func() {
			h.publishEventFn(connector.TopicPresence(cl.ID), true, evt)
			h.publishEventFn(connector.TopicPresenceEvent(cl.ID), false, evt)
		}()
	}
}

func toClientInfo(cl *mqtt.Client, connected bool,
	connectAt, disconnectAt *time.Time, err error) connector.ClientInfo {
	discReason := ""
	if err != nil {
		discReason = err.Error()
		if len(discReason) > 256 {
			discReason = discReason[0:256]
		}
	}
	res := connector.ClientInfo{
		ClientId:         cl.ID,
		Username:         string(cl.Properties.Username),
		Connected:        connected,
		DisconnectedAt:   disconnectAt,
		DisconnectReason: discReason,
		RemoteAddr:       cl.Net.Remote,
	}
	if connected {
		res.ConnectedAt = connectAt
	}
	return res
}

func toEvent(cl *mqtt.Client, typ string, t time.Time, err string) connector.Event {
	return connector.Event{
		EventType:        typ,
		Timestamp:        t.UnixMilli(),
		RemoteAddr:       cl.Net.Remote,
		ThingId:          string(cl.Properties.Username),
		DisconnectReason: err,
	}
}

func isPublishPresent(username string) bool {
	return !strings.HasPrefix(username, "$")
}
