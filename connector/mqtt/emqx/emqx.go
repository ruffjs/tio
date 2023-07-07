package emqx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	mq "ruff.io/tio/connector/mqtt/client"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	"ruff.io/tio/config"
	"ruff.io/tio/pkg/eventbus"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

const (
	// TopicClientConnected message example
	// 	{
	//     "username": "foo",
	//     "ts": 1625572213873,
	//     "sockport": 1883,
	//     "proto_ver": 4,
	//     "proto_name": "MQTT",
	//     "keepalive": 60,
	//     "ipaddress": "127.0.0.1",
	//     "expiry_interval": 0,
	//     "connected_at": 1625572213873,
	//     "connack": 0,
	//     "clientid": "emqtt-8348fe27a87976ad4db3",
	//     "clean_start": true
	// }
	// $SYS/brokers/${node}/clients/${clientid}/connected
	TopicClientConnected = "$SYS/brokers/+/clients/+/connected"

	// TopicClientDisconnected message example
	// 	{
	//     "username": "foo",
	//     "ts": 1625572213873,
	//     "sockport": 1883,
	//     "reason": "tcp_closed",
	//     "proto_ver": 4,
	//     "proto_name": "MQTT",
	//     "ipaddress": "127.0.0.1",
	//     "disconnected_at": 1625572213873,
	//     "clientid": "emqtt-8348fe27a87976ad4db3"
	// }
	// $SYS/brokers/${node}/clients/${clientid}/disconnected
	TopicClientDisconnected = "$SYS/brokers/+/clients/+/disconnected"

	presenceEventName = "presence"
)

type client struct {
	info     ClientInfo
	updateAt time.Time
}

// ClientInfo for emqx http api response
type ClientInfo struct {
	ClientId         string     `json:"clientid"`
	Username         string     `json:"username"`
	Connected        bool       `json:"connected"`
	ConnectedAt      *time.Time `json:"connected_at"`
	DisconnectedAt   *time.Time `json:"disconnected_at"`
	DisconnectReason string     `json:"-"`
	IpAddress        string     `json:"ip_address"`
	Port             int        `json:"port"`
}

type MqttConnectedEvent struct {
	ClientId    string `json:"clientid"`
	Username    string `json:"username"`
	IpAddress   string `json:"ipaddress"`
	Ts          int64  `json:"ts"`
	ConnectedAt int64  `json:"connected_at"`
}

type MqttDisconnectedEvent struct {
	ClientId       string `json:"clientid"`
	Username       string `json:"username"`
	IpAddress      string `json:"ipaddress"`
	Ts             int64  `json:"ts"`
	DisconnectedAt int64  `json:"disconnected_at"`
	Reason         string `json:"reason"`
}

type emqxAdapter struct {
	config           config.EmqxAdapterConfig
	mqttClient       mq.Client
	apiToken         string
	clients          sync.Map // map[string]client
	presenceEventBus *eventbus.EventBus[shadow.Event]
}

var _ shadow.Connectivity = (*emqxAdapter)(nil)

func NewEmqxAdapter(cfg config.EmqxAdapterConfig, mqCl mq.Client) shadow.Connectivity {
	return &emqxAdapter{
		config:           cfg,
		mqttClient:       mqCl,
		presenceEventBus: eventbus.NewEventBus[shadow.Event](),
	}
}

func (e *emqxAdapter) Start(ctx context.Context) error {
	e.apiToken = genAuthToken(e.config.ApiUser, e.config.ApiPassword)
	err := e.listenConnectivity(ctx)
	if err != nil {
		return err
	}
	e.initClientsInfo()

	// deprecated
	// cause tio mqtt client's config is clean_sessoin=false,
	// presence message of emqx will be received when tio up
	// e.initSyncPresence(ctx)

	return err
}

func (e *emqxAdapter) IsConnected(thingId string) (bool, error) {
	if cl, ok := e.clients.Load(thingId); ok {
		return cl.(client).info.Connected, nil
	}
	info, err := e.loadClientInfo(thingId)
	if err != nil {
		return false, err
	}
	return info.Connected, nil
}

func (e *emqxAdapter) OnConnect() <-chan shadow.Event {
	return e.presenceEventBus.Subscribe(presenceEventName)
}

func (e *emqxAdapter) ClientInfo(thingId string) (shadow.ClientInfo, error) {
	if cl, ok := e.clients.Load(thingId); ok {
		return toClientInfo(cl.(client).info), nil
	}
	info, err := e.loadClientInfo(thingId)
	if err != nil {
		return shadow.ClientInfo{}, err
	}
	return toClientInfo(*info), nil
}

func (e *emqxAdapter) AllClientInfo() ([]shadow.ClientInfo, error) {
	clients := make([]shadow.ClientInfo, 0)
	e.clients.Range(func(key, value any) bool {
		i := toClientInfo(value.(client).info)
		clients = append(clients, i)
		return true
	})
	return clients, nil
}

func toClientInfo(c ClientInfo) shadow.ClientInfo {
	return shadow.ClientInfo{
		ClientId:         c.ClientId,
		Username:         c.Username,
		Connected:        c.Connected,
		ConnectedAt:      c.ConnectedAt,
		DisconnectedAt:   c.DisconnectedAt,
		DisconnectReason: c.DisconnectReason,
		RemoteAddr:       c.IpAddress,
	}
}

func (e *emqxAdapter) Close(thingId string) error {
	return closeClient(e.config.ApiPrefix, e.apiToken, thingId)
}

func (e *emqxAdapter) Remove(thingId string) error {
	_ = e.Close(thingId)
	go func() {
		// wait for thing connection closed
		time.Sleep(time.Second)
		e.mqttClient.Publish(shadow.TopicPresence(thingId), 0, true, nil)
	}()
	return nil
}

func (e *emqxAdapter) loadClientInfo(thingId string) (*ClientInfo, error) {
	if cl, ok := e.getClient(thingId); ok {
		return &cl.info, nil
	}
	if c, err := fetchClient(e.config.ApiPrefix, e.apiToken, thingId); err == nil {
		e.updateClient(c)
		return &c, nil
	} else {
		return nil, err
	}
}

func (e *emqxAdapter) getClient(thingId string) (client, bool) {
	if cl, ok := e.clients.Load(thingId); ok {
		return cl.(client), ok
	}
	return client{}, false
}

func (e *emqxAdapter) initClientsInfo() {
	page := 1
	limit := 1000
	for {
		pageRes, err := fetchClientPage(e.config.ApiPrefix, e.apiToken, uint(page), uint(limit))
		if err != nil {
			log.Fatalf("init emqx clients info: %v", err)
		}
		for _, c := range pageRes.Data {
			e.updateClient(c)
		}
		if len(pageRes.Data) == 0 || pageRes.Meta.Count <= int64(pageRes.Meta.Page*pageRes.Meta.Limit) {
			break
		}
		page = page + 1
	}
}

func (e *emqxAdapter) updateClient(i ClientInfo) {
	if old, ok := e.clients.Load(i.ClientId); ok {
		// not the latest info, ignore it
		oldInfo := old.(client).info
		oldTime := oldInfo.ConnectedAt
		if oldInfo.DisconnectedAt != nil && oldInfo.ConnectedAt != nil &&
			oldInfo.DisconnectedAt.After(*oldInfo.ConnectedAt) {
			oldTime = oldInfo.DisconnectedAt
		}
		nowTime := i.ConnectedAt
		if !i.Connected {
			nowTime = i.DisconnectedAt
		}
		if oldTime != nil && nowTime.Before(*oldTime) {
			return
		}
		if !i.Connected {
			i.ConnectedAt = oldInfo.ConnectedAt
		}
	}
	e.clients.Store(i.ClientId, client{info: i, updateAt: time.Now()})
}

func (e *emqxAdapter) listenConnectivity(ctx context.Context) error {
	err := e.mqttClient.Subscribe(ctx, TopicClientConnected, 1, func(c mqtt.Client, message mqtt.Message) {
		go func() {
			log.Debugf("emqx connected %s", message.Payload())
			var d MqttConnectedEvent
			err := json.Unmarshal(message.Payload(), &d)
			if err != nil {
				log.Warnf("Unmarshal emqx mqtt client connected msg %q error: %v", message.Payload(), err)
				return
			}
			t := time.UnixMilli(d.ConnectedAt)
			e.updateClient(ClientInfo{
				ClientId:    d.ClientId,
				Username:    d.Username,
				Connected:   true,
				ConnectedAt: &t,
				IpAddress:   d.IpAddress,
			})
			evt := toConnectEvent(d)
			e.presenceEventBus.Publish(presenceEventName, evt)
			notifyEvent(ctx, e.mqttClient, d.ClientId, shadow.TopicPresence(d.ClientId), evt)
		}()
	})
	if err != nil {
		return errors.Wrapf(err, "subscribe emqx topic: %q", TopicClientConnected)
	}

	err = e.mqttClient.Subscribe(ctx, TopicClientDisconnected, 1, func(c mqtt.Client, message mqtt.Message) {
		go func() {
			log.Debugf("emqx disconnected %s", message.Payload())
			var d MqttDisconnectedEvent
			err := json.Unmarshal(message.Payload(), &d)
			if err != nil {
				log.Warnf("Unmarshal emqx mqtt client connected msg %q error: %v", message.Payload(), err)
				return
			}
			// Ignore disconnected event for these reasons (ref issue: https://askemq.com/t/topic/2358/4)
			if d.Reason == "discarded" || d.Reason == "takeovered" || d.Reason == "takenover" {
				log.Infof("Ignore client %q disconnected event for reason %q", d.ClientId, d.Reason)
				return
			}
			dt := time.UnixMilli(d.DisconnectedAt)
			e.updateClient(ClientInfo{
				ClientId:         d.ClientId,
				Username:         d.Username,
				Connected:        false,
				DisconnectedAt:   &dt,
				DisconnectReason: d.Reason,
				IpAddress:        d.IpAddress,
			})
			evt := toDisconnectEvent(d)
			e.presenceEventBus.Publish(presenceEventName, evt)
			notifyEvent(ctx, e.mqttClient, d.ClientId, shadow.TopicPresence(d.ClientId), evt)
		}()
	})
	if err != nil {
		return errors.Wrapf(err, "subscribe emqx topic: %q", TopicClientConnected)
	}
	return nil
}

func notifyEvent(ctx context.Context, mqCl mq.Client, clientId, topic string, evt shadow.Event) {
	if mq.IsSysClient(clientId) {
		log.Debugf("Ignored system mqtt client event %q", clientId)
		return
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		log.Errorf("Unmarshal event payload %#v: %v", evt, err)
		return
	}
	// if ctx.Err() != nil {
	// 	log.Warnf("Broker closed before notify")
	// 	return
	// }
	tk := mqCl.Publish(topic, 1, true, payload)
	tk.Wait()
	if tk.Error() != nil {
		log.Errorf("Publish %s event, topic=%q event=%#v error: %v",
			evt.EventType, topic, evt, tk.Error())
	} else {
		log.Infof("Published %s event, topic=%q event=%#v", evt.EventType, topic, evt)
	}
}

func genAuthToken(user, password string) string {
	tk := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
	return "Basic " + tk
}

func toConnectEvent(d MqttConnectedEvent) shadow.Event {
	evt := shadow.Event{
		EventType:  shadow.EventConnected,
		Timestamp:  d.ConnectedAt,
		ThingId:    d.Username,
		RemoteAddr: d.IpAddress,
	}
	return evt
}

func toDisconnectEvent(d MqttDisconnectedEvent) shadow.Event {
	evt := shadow.Event{
		EventType:        shadow.EventDisconnected,
		Timestamp:        d.DisconnectedAt,
		ThingId:          d.Username,
		RemoteAddr:       d.IpAddress,
		DisconnectReason: d.Reason,
	}
	return evt
}

func (e *emqxAdapter) initSyncPresence(ctx context.Context) {
	receivePresence(ctx, e.mqttClient)
	var preCtxCancel context.CancelFunc
	e.mqttClient.OnConnect(func() {
		// cancel previous context
		if preCtxCancel != nil {
			preCtxCancel()
		}
		subCtx, cancelCtx := context.WithCancel(ctx)
		preCtxCancel = cancelCtx

		syncPresence(subCtx, e.mqttClient, func(id string) (client, bool) {
			if c, ok := e.clients.Load(id); ok {
				return c.(client), true
			}
			return client{}, false
		}, func() map[string]client {
			m := make(map[string]client)
			e.clients.Range(func(k, v any) bool {
				m[k.(string)] = v.(client)
				return true
			})
			return m
		})
	})
}
