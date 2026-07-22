package source

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	ruleconnector "ruff.io/tio/rule/connector"
	rmodel "ruff.io/tio/rule/model"
)

const TypeMqtt = "mqtt"

func init() {
	Register(TypeMqtt, newMqtt)
}

type MqttConfig struct {
	Topic string `json:"topic"`
}

type mqttImpl struct {
	ctx      context.Context
	cancel   context.CancelFunc
	name     string
	config   MqttConfig
	conn     connector.Connector
	handlers sync.Map

	mu      sync.RWMutex
	started bool
	status  rmodel.StatusInfo
	metric  Metric
}

func newMqtt(ctx context.Context, name string, cfg map[string]any, _ ruleconnector.Conn, mainConn connector.Connector) (Source, error) {
	var ac MqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "decode config")
	}
	subCtx, cancel := context.WithCancel(ctx)
	m := &mqttImpl{
		ctx:    subCtx,
		cancel: cancel,
		name:   name,
		config: ac,
		conn:   mainConn,
		status: rmodel.StatusNotStarted(),
	}

	return m, nil
}

func (m *mqttImpl) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		slog.Info("Rule skip starting source (already started)", "type", TypeMqtt, "name", m.name)
		return nil
	}
	m.started = true

	if err := m.subscribe(); err != nil {
		m.status = rmodel.StatusDisconnected("failed to subscribe: "+err.Error(), err)
		return errors.WithMessagef(err, "failed to subscribe topic %q", m.config.Topic)
	}

	slog.Info("Rule started source", "type", TypeMqtt, "name", m.name)
	return nil
}

func (m *mqttImpl) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		slog.Info("Rule skip stopping source (not started)", "type", TypeMqtt, "name", m.name)
		return
	}
	m.started = false
	m.cancel()
	m.status = rmodel.StatusNotStarted()
	slog.Info("Rule stopped source", "type", TypeMqtt, "name", m.name)
}

func (m *mqttImpl) Status() rmodel.StatusInfo {
	m.status.Metric = Metric{Received: atomic.LoadInt64(&m.metric.Received)}
	return m.status
}

func (m *mqttImpl) Name() string {
	return m.name
}

func (*mqttImpl) Type() string {
	return TypeMqtt
}

func (m *mqttImpl) OnMsg(ruleName string, h MsgHander) {
	m.handlers.Store(ruleName, h)
	if h == nil {
		m.handlers.Delete(ruleName)
	}
}

func (m *mqttImpl) subscribe() error {
	queueName := "tio-rule-" + m.name
	return m.conn.QueueSubscribe(m.ctx, m.config.Topic, queueName, func(msg connector.Message) {
		if !m.started {
			return
		}

		atomic.AddInt64(&m.metric.Received, 1)

		topic := msg.Topic()
		thingId := extractThingIdFromTopic(topic)

		mm := Msg{
			ThingId: thingId,
			Topic:   topic,
			Payload: string(msg.Payload()),
		}

		m.handlers.Range(func(key, value any) bool {
			value.(MsgHander)(mm)
			return true
		})
	})
}

func extractThingIdFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) >= 3 && parts[0] == "$iothub" && parts[1] == "things" {
		return parts[2]
	}
	if len(parts) >= 4 && parts[0] == "$iothub" && parts[1] == "user" && parts[2] == "things" {
		return parts[3]
	}
	return ""
}
