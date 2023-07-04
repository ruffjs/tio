package embed

// mochi embedded mqtt broker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"ruff.io/tio/config"
	"ruff.io/tio/shadow"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/listeners"
	"github.com/mochi-co/mqtt/v2/system"
	"github.com/pkg/errors"
	"ruff.io/tio/pkg/eventbus"
	"ruff.io/tio/pkg/log"
)

const presenceEventName = "presence"

type AuthzFn func(user, password string) bool
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
	SuperUsers []config.UserPassword
}

var newOnce sync.Once
var broker *embedBroker

type Broker interface {
	Publish(topic string, payload []byte, retain bool, qos byte) error
	IsConnected(clientId string) bool
	OnConnect() <-chan shadow.Event
	ClientInfo(clientId string) (shadow.ClientInfo, error)
	Close() error
	CloseClient(clientId string) bool
	StatsInfo() *system.Info
}

func BrokerInstance() Broker {
	return broker
}

func InitBroker(c MochiConfig) Broker {
	newOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		evtBus := eventbus.NewEventBus[shadow.Event]()
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
	presenceEventBus *eventbus.EventBus[shadow.Event]

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

func (e *embedBroker) OnConnect() <-chan shadow.Event {
	return e.presenceEventBus.Subscribe(presenceEventName)
}

func (e *embedBroker) ClientInfo(clientId string) (shadow.ClientInfo, error) {
	if c, ok := e.clients.Load(clientId); ok {
		return c.(shadow.ClientInfo), nil
	}
	return shadow.ClientInfo{ClientId: clientId}, fmt.Errorf("not found")
}

func initBroker(ctx context.Context, cfg MochiConfig, evtBus *eventbus.EventBus[shadow.Event]) *mqtt.Server {
	svr := mqtt.New(nil)

	authHk := &authHook{authzFn: cfg.AuthzFn, aclFn: cfg.AclFn}
	err := svr.AddHook(authHk, nil)
	if err != nil {
		log.Fatalf("broker add hook: %v", err)
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

func (e *embedBroker) updateClient(c shadow.ClientInfo) {
	if old, ok := e.clients.Load(c.ClientId); ok {
		old := old.(shadow.ClientInfo)
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

func publishEventFn(e *mqtt.Server, evtBus *eventbus.EventBus[shadow.Event]) func(topic string, evt shadow.Event) {
	return func(topic string, evt shadow.Event) {
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
