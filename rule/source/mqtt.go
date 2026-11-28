package source

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/model"
	ruleconnector "ruff.io/tio/rule/connector"
	rmodel "ruff.io/tio/rule/model"
)

const TypeMqtt = "mqtt"

func init() {
	Register(TypeMqtt, newMqtt)
}

type MqttConfig struct {
	Topic string `json:"topic"`
	Qos   byte   `json:"qos"`
}

type mqttImpl struct {
	ctx      context.Context
	subCtx   context.Context
	cancel   context.CancelFunc
	name     string
	config   MqttConfig
	conn     *ruleconnector.Mqtt
	handlers sync.Map

	mu      sync.RWMutex
	started bool
	status  rmodel.StatusInfo
	metric  Metric
}

func newMqtt(ctx context.Context, name string, cfg map[string]any, ruleConn ruleconnector.Conn, _ connector.Connector) (Source, error) {
	var ac MqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "decode config")
	}
	mqttConn, ok := ruleConn.(*ruleconnector.Mqtt)
	if !ok {
		return nil, errors.New("wrong connector type for mqtt source")
	}
	m := &mqttImpl{
		ctx:    ctx,
		name:   name,
		config: ac,
		conn:   mqttConn,
		status: rmodel.StatusNotStarted(),
	}
	return m, nil
}

func (m *mqttImpl) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("Rule starting source", "type", TypeMqtt, "name", m.name)
	if m.started {
		slog.Info("Rule skip starting source (already started)", "type", TypeMqtt, "name", m.name)
		return nil
	}
	m.started = true
	m.subCtx, m.cancel = context.WithCancel(m.ctx)
	if err := m.subscribe(); err != nil {
		m.started = false
		m.cancel()
		m.status = rmodel.StatusDisconnected("failed to subscribe: "+err.Error(), err)
		return errors.WithMessagef(err, "failed to subscribe topic %q", m.config.Topic)
	}
	m.status = rmodel.StatusConnected()
	slog.Info("Rule started source", "type", TypeMqtt, "name", m.name)
	return nil
}

func (m *mqttImpl) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("Rule stopping source", "type", TypeMqtt, "name", m.name)
	if !m.started {
		slog.Info("Rule skip stopping source (not started)", "type", TypeMqtt, "name", m.name)
		return
	}
	m.started = false
	m.status = rmodel.StatusNotStarted()
	if err := m.conn.UnSubscribe(m.subCtx, m.config.Topic); err != nil {
		slog.Error("Rule source mqtt stop, failed to unsubscribe", "name", m.name, "error", err)
	}
	if m.cancel != nil {
		m.cancel()
	}
	slog.Info("Rule stopped source", "type", TypeMqtt, "name", m.name)
}

func (m *mqttImpl) Status() rmodel.StatusInfo {
	m.mu.RLock()
	started := m.started
	status := m.status
	m.mu.RUnlock()
	if !started {
		return rmodel.StatusNotStarted()
	}
	connSt := m.conn.Status()
	if connSt.Status == rmodel.Disconnected {
		return rmodel.StatusDisconnected("connector "+m.conn.Name()+" is disconnected: "+connSt.Reason, connSt.Error)
	}
	status.Metric = Metric{Received: atomic.LoadInt64(&m.metric.Received)}
	return status
}

func (m *mqttImpl) Name() string { return m.name }

func (*mqttImpl) Type() string { return TypeMqtt }

func (m *mqttImpl) OnMsg(ruleName string, h MsgHander) {
	m.handlers.Store(ruleName, h)
	if h == nil {
		m.handlers.Delete(ruleName)
	}
}

func (m *mqttImpl) subscribe() error {
	return m.conn.Subscribe(m.subCtx, m.config.Topic, m.config.Qos, func(_ mqtt.Client, msg mqtt.Message) {
		m.mu.RLock()
		started := m.started
		m.mu.RUnlock()
		if !started {
			return
		}
		atomic.AddInt64(&m.metric.Received, 1)
		thId, err := model.GetThingIdFromTopic(msg.Topic())
		if err != nil {
			slog.Error("Can't get thing id from topic in mqtt source subscription", "error", err)
		}
		mm := Msg{
			ThingId: thId,
			Topic:   msg.Topic(),
			Payload: string(msg.Payload()),
		}
		m.handlers.Range(func(_, value any) bool {
			value.(MsgHander)(mm)
			return true
		})
	})
}
