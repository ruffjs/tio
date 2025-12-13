package embed

// mochi embedded mqtt broker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"ruff.io/tio/connector"
	"ruff.io/tio/metrics"

	rv8 "github.com/go-redis/redis/v8"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage/badger"
	"github.com/mochi-mqtt/server/v2/hooks/storage/redis"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"
	"github.com/pkg/errors"
	"ruff.io/tio/config"
	"ruff.io/tio/pkg/eventbus"
	"ruff.io/tio/pkg/model"
)

const presenceEventName = "presence"

type ConnectParams packets.ConnectParams
type AuthzFn func(connParam ConnectParams) bool
type AclFn func(clientId, user string, topic string, write bool) bool
type MochiConfig struct {
	TcpPort             int
	TcpSslPort          int
	WsPort              int
	WssPort             int
	CertFile            string
	KeyFile             string
	AuthzFn             AuthzFn
	AclFn               AclFn
	Storage             config.InnerMqttStorage
	MessageQueueStorage config.MessageQueueStorage
	SuperUsers          []config.UserPassword
	MaximumInflight     uint16
}

var newOnce sync.Once
var broker *embedBroker

type Broker interface {
	Publish(topic string, payload []byte, retain bool, qos byte) error

	// callback function `cb` can't be blocked because of concurrent
	Subscribe(topic string, cb func(m Msg)) (subscriptionId int, err error)
	Unsubscribe(topic string, subscriptionId int) error

	IsConnected(clientId string) bool
	OnConnect() <-chan connector.PresenceEvent
	ClientInfo(clientId string) (connector.ClientInfo, error)
	AllClientInfo() ([]connector.ClientInfo, error)
	Close() error
	CloseClient(clientId string) bool
	StatsInfo() *system.Info
	AllClients() []Client
}

type Msg struct {
	ThingId string
	Topic   string
	Created int64
	Payload []byte
}

func BrokerInstance() Broker {
	return broker
}

func InitBroker(c MochiConfig) Broker {
	newOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		evtBus := eventbus.NewEventBus[connector.PresenceEvent]()
		s := initBroker(ctx, c, evtBus)
		broker = &embedBroker{
			impl:             s,
			presenceEventBus: evtBus,
			ctx:              ctx,
			cancel:           cancel,
		}

		// start
		err := s.Serve()
		if err != nil {
			slog.Error("Start embedded mqtt broker failed", "error", err)
			os.Exit(1)
		}

	})
	return broker
}

type embedBroker struct {
	impl             *mqtt.Server
	clients          sync.Map
	presenceEventBus *eventbus.EventBus[connector.PresenceEvent]

	ctx    context.Context
	cancel context.CancelFunc
}

func (e *embedBroker) StatsInfo() *system.Info {
	return broker.impl.Info
}

func (e *embedBroker) Publish(topic string, payload []byte, retain bool, qos byte) error {
	return e.impl.Publish(topic, payload, retain, qos)
}

var inlineSubIdCursor atomic.Int32

func (e *embedBroker) Subscribe(topic string, cb func(m Msg)) (subscriptionId int, err error) {
	// TODO generate subscriptionId ?
	// https://github.com/mochi-mqtt/server?tab=readme-ov-file#inline-subscribe
	// Note that only QoS 0 is supported for inline subscriptions.
	// If you wish to have multiple callbacks for the same filter,
	// you can use the MQTTv5 subscriptionId property to differentiate.
	subId := inlineSubIdCursor.Add(1)
	subscriptionId = int(subId)
	err = e.impl.Subscribe(topic, subscriptionId, func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
		thId, err := model.GetThingIdFromTopic(pk.TopicName)
		if err != nil {
			slog.Error("Can't get thing id from topic in embed mqtt broker subscription", "error", err)
		}
		cb(Msg{
			ThingId: thId,
			Topic:   pk.TopicName,
			Created: pk.Created,
			Payload: pk.Payload,
		})
	})
	return
}

func (e *embedBroker) Unsubscribe(topic string, subscriptionId int) error {
	return e.impl.Unsubscribe(topic, subscriptionId)
}

func (e *embedBroker) Close() error {
	e.cancel()
	err := e.impl.Close()
	return err
}

func (e *embedBroker) IsConnected(clientId string) bool {
	c, ok := e.impl.Clients.Get(clientId)
	if ok {
		return !c.Closed()
	}
	return false
}

func (e *embedBroker) OnConnect() <-chan connector.PresenceEvent {
	return e.presenceEventBus.Subscribe(presenceEventName)
}

func (e *embedBroker) ClientInfo(clientId string) (connector.ClientInfo, error) {
	if c, ok := e.clients.Load(clientId); ok {
		ci := c.(connector.ClientInfo)
		if cl, ok := e.impl.Clients.Get(clientId); ok {
			ci.Connected = !cl.Closed()
		} else {
			ci.Connected = false
		}
		return ci, nil
	}
	return connector.ClientInfo{ClientId: clientId}, fmt.Errorf("not found")
}

func (e *embedBroker) AllClientInfo() ([]connector.ClientInfo, error) {
	clients := make([]connector.ClientInfo, 0)
	mqttClients := e.impl.Clients.GetAll()
	e.clients.Range(func(key, value any) bool {
		i := value.(connector.ClientInfo)
		if c, ok := mqttClients[i.ClientId]; ok {
			i.Connected = !c.Closed()
		} else {
			i.Connected = false
		}
		clients = append(clients, i)
		return true
	})
	return clients, nil
}

func initBroker(ctx context.Context, cfg MochiConfig, evtBus *eventbus.EventBus[connector.PresenceEvent]) *mqtt.Server {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	opts := mqtt.Options{
		InlineClient:           true,
		SysTopicResendInterval: 5,
		Logger:                 logger,
	}
	opts.Capabilities = mqtt.NewDefaultServerCapabilities()
	if cfg.MaximumInflight > 0 {
		opts.Capabilities.MaximumInflight = cfg.MaximumInflight
	} else {
		opts.Capabilities.MaximumInflight = 1024 * 8
	}
	svr := mqtt.New(&opts)

	authHk := &authHook{authzFn: cfg.AuthzFn, aclFn: cfg.AclFn}
	err := svr.AddHook(authHk, nil)
	if err != nil {
		slog.Error("broker add hook", "error", err)
		os.Exit(1)
	}

	// 注册消息队列 Hook
	mqCfg := messageQueueConfig{
		StorageType: cfg.MessageQueueStorage.Type,
		StoragePath: cfg.MessageQueueStorage.FilePath,
	}
	if mqCfg.StorageType == "" {
		mqCfg.StorageType = "memory" // 默认使用内存存储
	}
	mqHook, err := newMessageQueueHook(svr, mqCfg)
	if err != nil {
		slog.Error("Failed to create message queue hook", "error", err)
		os.Exit(1)
	}
	err = svr.AddHook(mqHook, nil)
	if err != nil {
		slog.Error("Failed to add message queue hook", "error", err)
		os.Exit(1)
	}

	if cfg.Storage.Type == "file" && cfg.Storage.FilePath != "" {
		err = svr.AddHook(new(badger.Hook), &badger.Options{
			Path: cfg.Storage.FilePath,
		})
		if err != nil {
			slog.Error("Add storage badger hook", "error", err)
			os.Exit(1)
		} else {
			slog.Info("Add storage file badger hook")
		}
	} else if cfg.Storage.Type == "redis" {
		pre := "mqtt:"
		cPre := strings.TrimSpace(cfg.Storage.Redis.KeyPrefix)
		if cfg.Storage.Redis.KeyPrefix != "" {
			pre = cPre
		}
		err = svr.AddHook(new(redis.Hook), &redis.Options{
			HPrefix: pre,
			Options: &rv8.Options{
				Addr:     cfg.Storage.Redis.Addr,
				Password: cfg.Storage.Redis.Password,
				DB:       cfg.Storage.Redis.DB,
			},
		})
		if err != nil {
			slog.Error("Add storage redis hook", "error", err)
			os.Exit(1)
		} else {
			slog.Info("Add storage redis hook")
		}
	}

	presenceHk := &presenceHook{
		getClientFn:    getClientFn(svr),
		publishEventFn: publishEventFn(svr, evtBus),
	}
	err = svr.AddHook(presenceHk, nil)
	if err != nil {
		slog.Error("broker add hook", "error", err)
		os.Exit(1)
	}

	openMetricsHk := &metrics.OpenMetricsHook{}
	err = svr.AddHook(openMetricsHk, nil)
	if err != nil {
		slog.Error("broker add hook", "error", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", cfg.TcpPort)
	tcp := listeners.NewTCP(listeners.Config{ID: "tio-tcp", Type: "tcp", Address: addr})
	err = svr.AddListener(tcp)
	if err != nil {
		slog.Error("Start mqtt server add tcp listener failed", "error", err)
		os.Exit(1)
	}

	var cert tls.Certificate
	if cfg.TcpSslPort > 0 && cfg.KeyFile != "" && cfg.CertFile != "" {
		cert = readCert(cfg.KeyFile, cfg.CertFile)
		addr = fmt.Sprintf(":%d", cfg.TcpSslPort)
		tcpSsl := listeners.NewTCP(listeners.Config{
			ID:        "tio-tcp-ssl",
			Type:      "tcp",
			Address:   addr,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		})
		err = svr.AddListener(tcpSsl)
		if err != nil {
			slog.Error("Start mqtt server add ssl listener failed", "error", err)
			os.Exit(1)
		} else {
			slog.Info("Mqtt server tcp ssl listening", "addr", addr)
		}
	}

	if cfg.WssPort > 0 && cfg.KeyFile != "" && cfg.CertFile != "" {
		if cert.Certificate == nil {
			cert = readCert(cfg.KeyFile, cfg.CertFile)
		}
		addr = fmt.Sprintf(":%d", cfg.WssPort)
		wss := listeners.NewTCP(listeners.Config{
			ID:        "tio-wss",
			Type:      "ws",
			Address:   addr,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		})
		err = svr.AddListener(wss)
		if err != nil {
			slog.Error("Start mqtt server add wss listener failed", "error", err)
			os.Exit(1)
		} else {
			slog.Info("Mqtt server wss listening", "addr", addr)
		}
	}

	wsAddr := fmt.Sprintf(":%d", cfg.WsPort)
	ws := listeners.NewWebsocket(listeners.Config{ID: "tio-ws", Type: "ws", Address: wsAddr})
	err = svr.AddListener(ws)
	if err != nil {
		slog.Error("Add mqtt broker websocket listener failed", "error", err)
		os.Exit(1)
	}

	return svr
}

func readCert(keyFile, certFile string) tls.Certificate {
	keyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		slog.Error("Read key file", "error", err)
		os.Exit(1)
	}
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		slog.Error("Read cert file", "error", err)
		os.Exit(1)
	}
	cert, err := tls.X509KeyPair(keyBytes, certBytes)
	if err != nil {
		slog.Error("Wrong cert or key file", "error", err)
		os.Exit(1)
	}
	return cert
}

// updateClient sync mqtt client info, cause mochi-mqtt has no connect time for client
func (e *embedBroker) updateClient(c connector.ClientInfo) {
	if old, ok := e.clients.Load(c.ClientId); ok {
		old := old.(connector.ClientInfo)
		// not the latest info, ignore it
		oldTime := old.ConnectedAt
		if old.DisconnectedAt != nil &&
			(old.ConnectedAt == nil || old.DisconnectedAt.After(*old.ConnectedAt)) {
			oldTime = old.DisconnectedAt
		}
		newTime := c.ConnectedAt
		if !c.Connected {
			newTime = c.DisconnectedAt
		}
		if oldTime != nil && newTime.Before(*oldTime) {
			return
		}

		if !c.Connected {
			// copy the last connected time for disconnected client
			c.ConnectedAt = old.ConnectedAt
		}
	}
	e.clients.Store(c.ClientId, c)
}

func (e *embedBroker) CloseClient(clientId string) bool {
	c, ok := e.impl.Clients.Get(clientId)
	if ok {
		c.Stop(errors.New("manual close"))
		slog.Info("Closed mqtt client", "clientId", clientId)
		return true
	}
	return false
}

func publishEventFn(e *mqtt.Server, evtBus *eventbus.EventBus[connector.PresenceEvent]) func(thingId string, evt connector.PresenceEvent) {
	return func(thingId string, evt connector.PresenceEvent) {
		payload, err := json.Marshal(evt)
		if err != nil {
			slog.Error("Unmarshal event payload", "event", evt, "error", err)
			return
		}
		evtBus.Publish(presenceEventName, evt)

		topic := connector.TopicPresence(thingId)
		err = e.Publish(topic, payload, true, 1)
		if err != nil {
			slog.Error("Publish presence", "topic", topic, "event", evt, "error", err)
		} else {
			slog.Info("Published presence", "topic", topic, "event", evt)
		}

		topic = connector.TopicPresenceEvent(thingId)
		err = e.Publish(topic, payload, false, 1)
		if err != nil {
			slog.Error("Publish presence event", "topic", topic, "event", evt, "error", err)
		} else {
			slog.Info("Published presence event", "topic", topic, "event", evt)
		}
	}
}

func getClientFn(e *mqtt.Server) func(id string) (*mqtt.Client, bool) {
	return func(id string) (*mqtt.Client, bool) {
		return e.Clients.Get(id)
	}
}

// This api is not for integration, it 's for temporary debugging
// TODO:
//   - rethink the api
//   - change 'Client' json field name to lowercase camel
func (e *embedBroker) AllClients() []Client {
	l := e.impl.Clients.GetAll()
	rl := []Client{}
	for _, c := range l {
		sbs := []packets.Subscription{}
		for _, s := range c.State.Subscriptions.GetAll() {
			sbs = append(sbs, s)
		}
		rc := Client{
			Properties: ClientProperties{
				Will:            c.Properties.Will,
				Username:        c.Properties.Username,
				ProtocolVersion: c.Properties.ProtocolVersion,
				Clean:           c.Properties.Clean,
			},
			State: ClientState{
				StopCause:       c.StopCause(),
				InflightSize:    c.State.Inflight.Len(),
				Subscriptions:   sbs,
				Keepalive:       c.State.Keepalive,
				ServerKeepalive: c.State.ServerKeepalive,
			},
			Net: c.Net,
			ID:  c.ID,
		}
		rl = append(rl, rc)
	}
	return rl
}

type Client struct {
	Properties ClientProperties
	State      ClientState
	Net        mqtt.ClientConnection
	ID         string
}

type ClientState struct {
	// TopicAliases    mqtt.TopicAliases
	StopCause       error
	InflightSize    int
	Subscriptions   []packets.Subscription
	Keepalive       uint16
	ServerKeepalive bool
}

type ClientProperties struct {
	// Props           packets.Properties
	Will            mqtt.Will
	Username        []byte
	ProtocolVersion byte
	Clean           bool
}
