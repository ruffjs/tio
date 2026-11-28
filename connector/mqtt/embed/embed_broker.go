package embed

// embedded comqtt broker adapter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"

	redisv9 "github.com/redis/go-redis/v9"
	mqtt "github.com/wind-c/comqtt/v2/mqtt"
	"github.com/wind-c/comqtt/v2/mqtt/hooks/storage/badger"
	"github.com/wind-c/comqtt/v2/mqtt/hooks/storage/redis"
	"github.com/wind-c/comqtt/v2/mqtt/listeners"
	"github.com/wind-c/comqtt/v2/mqtt/packets"
	"github.com/wind-c/comqtt/v2/mqtt/system"
)

var _ connector.Connector = (*Broker)(nil)

type Broker struct {
	opt Options

	svr *mqtt.Server

	authzFn AuthzFn
	aclFn   AclFn

	codec  codec.Codec
	bridge Bridge

	// clients caches connector.ClientInfo keyed by thingId. Populated by the
	// presence hook; Connected is always refreshed from the live client.
	clients sync.Map

	presenceHandlers []connector.PresenceHandler
	presenceMu       sync.Mutex

	presenceEpoch    int64
	presenceRevision atomic.Uint64

	subId atomic.Int32
}

func (b *Broker) ConfigureAuth(authzFn AuthzFn, aclFn AclFn) error {
	b.authzFn = authzFn
	b.aclFn = aclFn
	return nil
}

func (b *Broker) Start(ctx context.Context) error {
	if b.authzFn == nil || b.aclFn == nil {
		return errors.New("connector not configured: call ConfigureAuth first")
	}

	svr := mqtt.New(&mqtt.Options{
		InlineClient:           true,
		SysTopicResendInterval: 5,
		Logger:                 slog.Default(),
	})
	b.svr = svr
	b.presenceEpoch = time.Now().UnixMicro()

	if err := svr.AddHook(&authHook{authzFn: b.authzFn, aclFn: b.aclFn}, nil); err != nil {
		return fmt.Errorf("add auth hook: %w", err)
	}
	if err := svr.AddHook(&presenceHook{b: b}, nil); err != nil {
		return fmt.Errorf("add presence hook: %w", err)
	}
	if err := svr.AddHook(&publishHook{b: b}, nil); err != nil {
		return fmt.Errorf("add publish hook: %w", err)
	}

	// Listeners before storage: a bad cert fails here and leaves nothing open,
	// whereas a badger/redis store opened first would leak on that error.
	if err := b.addListeners(svr); err != nil {
		return err
	}

	if err := b.addStorageHook(svr); err != nil {
		_ = svr.Close() // release listeners bound before the store failed
		return err
	}

	if err := svr.Serve(); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func (b *Broker) addStorageHook(svr *mqtt.Server) error {
	st := b.opt.Storage
	switch {
	case st.Type == "file" && st.FilePath != "":
		if err := svr.AddHook(new(badger.Hook), &badger.Options{Path: st.FilePath}); err != nil {
			return fmt.Errorf("add badger storage hook: %w", err)
		}
		slog.Info("add storage badger hook", "path", st.FilePath)
	case st.Type == "redis":
		pre := "mqtt:"
		if st.Redis.KeyPrefix != "" {
			pre = strings.TrimSpace(st.Redis.KeyPrefix)
		}
		err := svr.AddHook(new(redis.Hook), &redis.Options{
			HPrefix: pre,
			Options: &redisv9.Options{
				Addr:     st.Redis.Addr,
				Password: st.Redis.Password,
				DB:       st.Redis.DB,
			},
		})
		if err != nil {
			return fmt.Errorf("add redis storage hook: %w", err)
		}
		slog.Info("add storage redis hook", "prefix", pre, "addr", st.Redis.Addr)
	}
	return nil
}

func (b *Broker) addListeners(svr *mqtt.Server) error {
	if b.opt.TcpPort > 0 {
		tcp := listeners.NewTCP("tio-tcp", fmt.Sprintf(":%d", b.opt.TcpPort), nil)
		if err := svr.AddListener(tcp); err != nil {
			return fmt.Errorf("add tcp listener: %w", err)
		}
	}

	if b.opt.TcpSslPort > 0 && b.opt.KeyFile != "" && b.opt.CertFile != "" {
		cert, err := readCert(b.opt.KeyFile, b.opt.CertFile)
		if err != nil {
			return fmt.Errorf("load tcp-ssl cert: %w", err)
		}
		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		if b.opt.ClientCAFile != "" {
			pool, err := readCACertPool(b.opt.ClientCAFile)
			if err != nil {
				return fmt.Errorf("load tcp-ssl client ca: %w", err)
			}
			tlsCfg.ClientCAs = pool
			tlsCfg.ClientAuth = tls.RequestClientCert
			if b.opt.RequireClientCert {
				tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
			}
		}
		tcpSsl := listeners.NewTCP("tio-tcp-ssl", fmt.Sprintf(":%d", b.opt.TcpSslPort), &listeners.Config{
			TLSConfig: tlsCfg,
		})
		if err := svr.AddListener(tcpSsl); err != nil {
			return fmt.Errorf("add tcp-ssl listener: %w", err)
		}
	}

	if b.opt.WsPort > 0 {
		ws := listeners.NewWebsocket("tio-ws", fmt.Sprintf(":%d", b.opt.WsPort), nil)
		if err := svr.AddListener(ws); err != nil {
			return fmt.Errorf("add ws listener: %w", err)
		}
	}

	if b.opt.WssPort > 0 && b.opt.KeyFile != "" && b.opt.CertFile != "" {
		cert, err := readCert(b.opt.KeyFile, b.opt.CertFile)
		if err != nil {
			return fmt.Errorf("load wss cert: %w", err)
		}
		wss := listeners.NewWebsocket("tio-wss", fmt.Sprintf(":%d", b.opt.WssPort), &listeners.Config{
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		})
		if err := svr.AddListener(wss); err != nil {
			return fmt.Errorf("add wss listener: %w", err)
		}
	}

	return nil
}

func readCert(keyFile, certFile string) (tls.Certificate, error) {
	keyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("read key file %q: %w", keyFile, err)
	}
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("read cert file %q: %w", certFile, err)
	}
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse cert/key pair: %w", err)
	}
	return cert, nil
}

func readCACertPool(caFile string) (*x509.CertPool, error) {
	caBytes, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read ca file %q: %w", caFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("no certificates parsed from %q", caFile)
	}
	return pool, nil
}

func (b *Broker) Publish(topic string, payload []byte) error {
	return b.svr.Publish(topic, payload, false, 0)
}

func (b *Broker) PublishReliable(topic string, payload []byte) error {
	return b.svr.Publish(topic, payload, false, 1)
}

func (b *Broker) PublishRetained(topic string, payload []byte) error {
	return b.svr.Publish(topic, payload, true, 1)
}

type msg struct {
	topic   string
	payload []byte
}

func (m *msg) Topic() string   { return m.topic }
func (m *msg) Payload() []byte { return m.payload }

func (b *Broker) Subscribe(ctx context.Context, topic string, cb func(connector.Message)) error {
	subId := int(b.subId.Add(1))
	if err := b.svr.Subscribe(topic, subId, func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
		cb(&msg{topic: pk.TopicName, payload: pk.Payload})
	}); err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		_ = b.svr.Unsubscribe(topic, subId)
	}()
	return nil
}

// QueueSubscribe degenerates to Subscribe: with a single node there is only one
// consumer, so queue-group semantics are equivalent.
func (b *Broker) QueueSubscribe(ctx context.Context, topic, queue string, cb func(connector.Message)) error {
	return b.Subscribe(ctx, topic, cb)
}

// IsConnected reports true if any live client has thingId as its authenticated
// principal (username). Several clients may share a username and a closed client
// can remain in svr.Clients briefly, so every match is checked for liveness.
func (b *Broker) IsConnected(thingId string) (bool, error) {
	return b.anyLiveClient(thingId), nil
}

// anyLiveClient reports whether any live client has thingId as its
// authenticated principal (username).
func (b *Broker) anyLiveClient(thingId string) bool {
	for _, cl := range b.svr.Clients.GetAll() {
		if string(cl.Properties.Username) == thingId && !cl.Closed() {
			return true
		}
	}
	return false
}

func (b *Broker) ClientInfo(thingId string) (connector.ClientInfo, error) {
	if c, ok := b.clients.Load(thingId); ok {
		ci := c.(connector.ClientInfo)
		ci.Connected = b.anyLiveClient(thingId)
		return ci, nil
	}
	return connector.ClientInfo{ClientId: thingId}, fmt.Errorf("%w for %q", connector.ErrConnectionNotFound, thingId)
}

func (b *Broker) AllClientInfo() ([]connector.ClientInfo, error) {
	clients := make([]connector.ClientInfo, 0)
	b.clients.Range(func(_, value any) bool {
		ci := value.(connector.ClientInfo)
		ci.Connected = b.anyLiveClient(ci.Username)
		clients = append(clients, ci)
		return true
	})
	return clients, nil
}

func (b *Broker) Close(thingId string) error {
	var closed bool
	for _, cl := range b.svr.Clients.GetAll() {
		if string(cl.Properties.Username) == thingId && !cl.Closed() {
			cl.Stop(errors.New("manual close"))
			closed = true
		}
	}
	if !closed {
		return fmt.Errorf("%w for %q", connector.ErrConnectionNotFound, thingId)
	}
	return nil
}

// CloseAll closes the thing's connections but never errors when none match:
// REST kick allows repeated calls.
func (b *Broker) CloseAll(thingId string) error {
	for _, cl := range b.svr.Clients.GetAll() {
		if string(cl.Properties.Username) == thingId {
			cl.Stop(errors.New("manual close"))
		}
	}
	return nil
}

func (b *Broker) Remove(thingId string) error {
	if err := b.Close(thingId); err != nil && !errors.Is(err, connector.ErrConnectionNotFound) {
		return err
	}
	go func() {
		// wait for the thing connection to close
		time.Sleep(time.Second)
		_ = b.PublishRetained(connector.TopicPresence(thingId), nil)
	}()
	return nil
}

func (b *Broker) OnLocalPresence(h connector.PresenceHandler) {
	b.presenceMu.Lock()
	defer b.presenceMu.Unlock()
	b.presenceHandlers = append(b.presenceHandlers, h)
}

// publishPresence records the client's presence in the local cache and emits
// the presence event to MQTT, local handlers and (if enabled) the bridge.
func (b *Broker) publishPresence(ci connector.ClientInfo) {
	if !isPublishPresent(ci.Username) {
		return
	}
	ci.PresenceEpoch = b.presenceEpoch
	ci.PresenceRevision = b.presenceRevision.Add(1)
	b.clients.Store(ci.Username, ci)

	evt := connector.PresenceEvent{
		ThingId:          ci.Username,
		ClientId:         ci.ClientId,
		RemoteAddr:       ci.RemoteAddr,
		DisconnectReason: ci.DisconnectReason,
	}
	var ts *time.Time
	if ci.Connected {
		evt.EventType = connector.EventConnected
		ts = ci.ConnectedAt
	} else {
		evt.EventType = connector.EventDisconnected
		ts = ci.DisconnectedAt
	}
	if ts != nil {
		evt.Timestamp = ts.UnixMilli()
		evt.TimestampUs = ts.UnixMicro()
	}

	var payload []byte
	if b.codec != nil {
		var err error
		payload, err = b.codec.Marshal(evt)
		if err != nil {
			slog.Error("marshal presence event", "thingId", ci.Username, "error", err)
			return
		}
	}

	if err := b.svr.Publish(connector.TopicPresence(ci.Username), payload, true, 1); err != nil {
		slog.Error("publish retained presence", "thingId", ci.Username, "error", err)
	}
	if err := b.svr.Publish(connector.TopicPresenceEvent(ci.Username), payload, false, 1); err != nil {
		slog.Error("publish presence event", "thingId", ci.Username, "error", err)
	}

	b.presenceMu.Lock()
	handlers := append([]connector.PresenceHandler(nil), b.presenceHandlers...)
	b.presenceMu.Unlock()
	for _, h := range handlers {
		h(ci)
	}

	if b.bridge != nil {
		b.bridge.ForwardPresence(evt, payload)
	}
}

func (b *Broker) AllClients() []Client {
	if b.svr == nil {
		return nil
	}
	l := b.svr.Clients.GetAll()
	rl := make([]Client, 0, len(l))
	for _, c := range l {
		sbs := make([]packets.Subscription, 0)
		for _, s := range c.State.Subscriptions.GetAll() {
			sbs = append(sbs, s)
		}
		rl = append(rl, Client{
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
		})
	}
	return rl
}

func (b *Broker) StatsInfo() *system.Info {
	if b.svr == nil {
		return nil
	}
	return b.svr.Info
}

type Client struct {
	Properties ClientProperties
	State      ClientState
	Net        mqtt.ClientConnection
	ID         string
}

type ClientState struct {
	StopCause       error
	InflightSize    int
	Subscriptions   []packets.Subscription
	Keepalive       uint16
	ServerKeepalive bool
}

type ClientProperties struct {
	Will            mqtt.Will
	Username        []byte
	ProtocolVersion byte
	Clean           bool
}
