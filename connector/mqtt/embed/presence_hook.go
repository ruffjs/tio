package embed

import (
	"bytes"
	"fmt"
	"ruff.io/tio/connector"
	"strings"
	"time"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/packets"
	"ruff.io/tio/pkg/log"
)

type presenceHook struct {
	mqtt.HookBase
	publishEventFn func(topic string, evt connector.Event)
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
	log.Debugf("Mqtt OnConnect now=%d clientId=%s username=%s ip=%s, packet=%#v",
		time.Now().UnixNano(), cl.ID, cl.Properties.Username, cl.Net.Remote, pk)
	exist, ok := h.getClientFn(cl.ID)
	if !ok || exist.Closed() {
		log.Debugf("Ignore OnConnect message "+
			"cause client is disconnected,"+
			" may be concurrent connect and disconnect. clientId=%q username=%q",
			cl.ID, cl.Properties.Username)
		return
	}
	now := time.Now()
	cinfo := toClientInfo(cl, true, &now, nil, nil)
	broker.updateClient(cinfo)
	if isPublishPresent(string(cl.Properties.Username)) {
		evt := toEvent(cl, connector.EventConnected, now, "")
		go h.publishEventFn(connector.TopicPresence(cl.ID), evt)
	}
}

func (h *presenceHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	log.Debugf("Mqtt OnDisconnect now=%d clientId=%s username=%s ip=%s, error=%v",
		time.Now().UnixNano(), cl.ID, cl.Properties.Username, cl.Net.Remote, err)

	exist, ok := h.getClientFn(cl.ID)
	if ok && !exist.Closed() {
		log.Debugf("Ignore OnDisconnect message, "+
			"cause client is connected, may be concurrent connect and disconnect. "+
			"clientId=%q username=%q,"+
			cl.ID, cl.Properties.Username)
		return
	}
	now := time.Now()
	cinfo := toClientInfo(cl, false, nil, &now, err)
	broker.updateClient(cinfo)
	if isPublishPresent(string(cl.Properties.Username)) {
		evt := toEvent(cl, connector.EventDisconnected, now, fmt.Sprintf("%s", err))
		go h.publishEventFn(connector.TopicPresence(cl.ID), evt)
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
