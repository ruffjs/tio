package embed

// mochi embedded mqtt broker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"ruff.io/tio/connector"

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
	"ruff.io/tio/pkg/log"
)

const presenceEventName = "presence"

type ConnectParams packets.ConnectParams
type AuthzFn func(connParam ConnectParams) bool
type AclFn func(user string, topic string, write bool) bool
type MochiConfig struct {
	TcpPort    int
	TcpSslPort int
	WsPort     int
	WssPort    int
	CertFile   string
	KeyFile    string
	AuthzFn    AuthzFn
	AclFn      AclFn
	Storage    config.InnerMqttStorage
	SuperUsers []config.UserPassword
}

var newOnce sync.Once
var broker *embedBroker

type Broker interface {
	Publish(topic string, payload []byte, retain bool, qos byte) error
	IsConnected(clientId string) bool
	OnConnect() <-chan connector.Event
	ClientInfo(clientId string) (connector.ClientInfo, error)
	AllClientInfo() ([]connector.ClientInfo, error)
	Close() error
	CloseClient(clientId string) bool
	StatsInfo() *system.Info
	AllClients() []Client
}

func BrokerInstance() Broker {
	return broker
}

func InitBroker(c MochiConfig) Broker {
	newOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		evtBus := eventbus.NewEventBus[connector.Event]()
		s := initBroker(ctx, c, evtBus)
		broker = &embedBroker{
			impl:             s,
			presenceEventBus: evtBus,
			ctx:              ctx,
			cancel:           cancel}
	})
	return broker
}

type embedBroker struct {
	impl             *mqtt.Server
	clients          sync.Map // map[string]shadow.ClientInfo
	presenceEventBus *eventbus.EventBus[connector.Event]

	ctx    context.Context
	cancel context.CancelFunc
}

func (e *embedBroker) StatsInfo() *system.Info {
	return broker.impl.Info
}

func (e *embedBroker) Publish(topic string, payload []byte, retain bool, qos byte) error {
	return e.impl.Publish(topic, payload, retain, qos)
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

func (e *embedBroker) OnConnect() <-chan connector.Event {
	return e.presenceEventBus.Subscribe(presenceEventName)
}

func (e *embedBroker) ClientInfo(clientId string) (connector.ClientInfo, error) {
	if c, ok := e.clients.Load(clientId); ok {
		return c.(connector.ClientInfo), nil
	}
	return connector.ClientInfo{ClientId: clientId}, fmt.Errorf("not found")
}

func (e *embedBroker) AllClientInfo() ([]connector.ClientInfo, error) {
	clients := make([]connector.ClientInfo, 0)
	e.clients.Range(func(key, value any) bool {
		i := value.(connector.ClientInfo)
		clients = append(clients, i)
		return true
	})
	return clients, nil
}

func initBroker(ctx context.Context, cfg MochiConfig, evtBus *eventbus.EventBus[connector.Event]) *mqtt.Server {
	svr := mqtt.New(&mqtt.Options{
		InlineClient: true,
	})

	authHk := &authHook{authzFn: cfg.AuthzFn, aclFn: cfg.AclFn}
	err := svr.AddHook(authHk, nil)
	if err != nil {
		log.Fatalf("broker add hook: %v", err)
	}

	if cfg.Storage.Type == "file" && cfg.Storage.FilePath != "" {
		err = svr.AddHook(new(badger.Hook), &badger.Options{
			Path: cfg.Storage.FilePath,
		})
		if err != nil {
			log.Fatalf("Add storage badger hook: %v", err)
		} else {
			log.Infof("Add storage file badger hook")
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
			log.Fatalf("Add storage redis hook: %v", err)
		} else {
			log.Info("Add storage redis hook")
		}
	}

	presenceHk := &presenceHook{
		getClientFn:    getClientFn(svr),
		publishEventFn: publishEventFn(svr, evtBus),
	}
	err = svr.AddHook(presenceHk, nil)
	if err != nil {
		log.Fatalf("broker add hook: %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.TcpPort)
	tcp := listeners.NewTCP("tio-tcp", addr, nil)
	err = svr.AddListener(tcp)
	if err != nil {
		log.Fatalf("Start mqtt server add tcp listener failed: %v", err)
	}

	var cert tls.Certificate
	if cfg.TcpSslPort > 0 && cfg.KeyFile != "" && cfg.CertFile != "" {
		cert = readCert(cfg.KeyFile, cfg.CertFile)
		addr = fmt.Sprintf(":%d", cfg.TcpSslPort)
		tcpSsl := listeners.NewTCP("tio-tcp-ssl", addr,
			&listeners.Config{TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}})
		err = svr.AddListener(tcpSsl)
		if err != nil {
			log.Fatalf("Start mqtt server add ssl listener failed: %v", err)
		} else {
			log.Infof("Mqtt server tcp ssl listening on %s", addr)
		}
	}

	if cfg.WssPort > 0 && cfg.KeyFile != "" && cfg.CertFile != "" {
		if cert.Certificate == nil {
			cert = readCert(cfg.KeyFile, cfg.CertFile)
		}
		addr = fmt.Sprintf(":%d", cfg.WssPort)
		wss := listeners.NewWebsocket("tio-wss", addr,
			&listeners.Config{TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}})
		err = svr.AddListener(wss)
		if err != nil {
			log.Fatalf("Start mqtt server add wss listener failed: %v", err)
		} else {
			log.Infof("Mqtt server wss listening on %s", addr)
		}
	}

	wsAddr := fmt.Sprintf(":%d", cfg.WsPort)
	ws := listeners.NewWebsocket("tio-ws", wsAddr, nil)
	err = svr.AddListener(ws)
	if err != nil {
		log.Fatalf("Add mqtt broker websocket listener failed: %v", err)
	}

	go func() {
		err = svr.Serve()
		if err != nil {
			log.Fatalf("Start embedded mqtt broker failed: %v", err)
		}
		log.Infof("Started embedded mqtt broker, listening on %s", addr)
	}()

	return svr
}

func readCert(keyFile, certFile string) tls.Certificate {
	keyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		log.Fatalf("Read key file %v", err)
	}
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		log.Fatalf("Read cert file %v", err)
	}
	cert, err := tls.X509KeyPair(keyBytes, certBytes)
	if err != nil {
		log.Fatalf("Wrong cert or key file: %v", err)
	}
	return cert
}

func (e *embedBroker) updateClient(c connector.ClientInfo) {
	if old, ok := e.clients.Load(c.ClientId); ok {
		old := old.(connector.ClientInfo)
		// not the latest info, ignore it
		oldTime := old.ConnectedAt
		if old.DisconnectedAt != nil && old.ConnectedAt != nil &&
			old.DisconnectedAt.After(*old.ConnectedAt) {
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
			c.ConnectedAt = old.ConnectedAt
		}
	}
	e.clients.Store(c.ClientId, c)
}

func (e *embedBroker) CloseClient(clientId string) bool {
	c, ok := e.impl.Clients.Get(clientId)
	if ok {
		c.Stop(errors.New("manual close"))
		log.Infof("Closed mqtt client: clientId=%s", clientId)
		return true
	}
	return false
}

func publishEventFn(e *mqtt.Server, evtBus *eventbus.EventBus[connector.Event]) func(topic string, evt connector.Event) {
	return func(topic string, evt connector.Event) {
		payload, err := json.Marshal(evt)
		if err != nil {
			log.Errorf("Unmarshal event payload %#v: %v", evt, err)
			return
		}
		evtBus.Publish(presenceEventName, evt)
		err = e.Publish(topic, payload, true, 1)
		if err != nil {
			log.Errorf("Publish %s event %#v error: %v", evt.EventType, evt, err)
		} else {
			log.Infof("Published %s event, topic=%q event=%v", evt.EventType, topic, evt)
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
