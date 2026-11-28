package source

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/pkg/protocol"
	ruleconnector "ruff.io/tio/rule/connector"
	rmodel "ruff.io/tio/rule/model"
)

const TypeEmbedMqtt = "embed-mqtt"

func init() {
	Register(TypeEmbedMqtt, newEmbedMqtt)
}

type EmbedMqttConfig struct {
	Topic string `json:"topic"`
}

type embedMqttImpl struct {
	parent   context.Context
	ctx      context.Context
	cancel   context.CancelFunc
	name     string
	config   EmbedMqttConfig
	conn     connector.Connector
	handlers sync.Map

	mu      sync.Mutex
	started atomic.Bool
	status  rmodel.StatusInfo
	metric  Metric
}

func newEmbedMqtt(ctx context.Context, name string, cfg map[string]any, _ ruleconnector.Conn, mainConn connector.Connector) (Source, error) {
	var ac EmbedMqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "decode config")
	}
	m := &embedMqttImpl{
		parent: ctx,
		name:   name,
		config: ac,
		conn:   mainConn,
		status: rmodel.StatusNotStarted(),
	}
	return m, nil
}

func (m *embedMqttImpl) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started.Load() {
		slog.Info("Rule skip starting source (already started)", "type", TypeEmbedMqtt, "name", m.name)
		return nil
	}
	m.ctx, m.cancel = context.WithCancel(m.parent)
	m.started.Store(true)

	if err := m.subscribe(); err != nil {
		m.started.Store(false)
		m.cancel()
		m.status = rmodel.StatusDisconnected("failed to subscribe: "+err.Error(), err)
		return errors.WithMessagef(err, "failed to subscribe topic %q", m.config.Topic)
	}

	m.status = rmodel.StatusConnected()
	slog.Info("Rule started source", "type", TypeEmbedMqtt, "name", m.name)
	return nil
}

func (m *embedMqttImpl) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started.Load() {
		slog.Info("Rule skip stopping source (not started)", "type", TypeEmbedMqtt, "name", m.name)
		return
	}
	m.started.Store(false)
	if m.cancel != nil {
		m.cancel()
	}
	m.status = rmodel.StatusNotStarted()
	slog.Info("Rule stopped source", "type", TypeEmbedMqtt, "name", m.name)
}

func (m *embedMqttImpl) Status() rmodel.StatusInfo {
	m.mu.Lock()
	st := m.status
	m.mu.Unlock()
	st.Metric = Metric{Received: atomic.LoadInt64(&m.metric.Received)}
	return st
}

func (m *embedMqttImpl) Name() string { return m.name }

func (*embedMqttImpl) Type() string { return TypeEmbedMqtt }

func (m *embedMqttImpl) OnMsg(ruleName string, h MsgHander) {
	m.handlers.Store(ruleName, h)
	if h == nil {
		m.handlers.Delete(ruleName)
	}
}

func (m *embedMqttImpl) subscribe() error {
	return m.conn.Subscribe(m.ctx, m.config.Topic, func(msg connector.Message) {
		if !m.started.Load() {
			return
		}
		atomic.AddInt64(&m.metric.Received, 1)

		topic := msg.Topic()
		mm := Msg{
			ThingId: thingIdFromTopic(topic),
			Topic:   topic,
			Payload: string(msg.Payload()),
		}
		m.handlers.Range(func(_, value any) bool {
			value.(MsgHander)(mm)
			return true
		})
	})
}

func thingIdFromTopic(topic string) string {
	if strings.HasPrefix(topic, protocol.TopicSimplePrefix) {
		if id, _, err := protocol.ParseTopicDir(topic); err == nil {
			return id
		}
	}
	if id, err := model.GetThingIdFromTopic(topic); err == nil {
		return id
	}
	return ""
}
