package source

import (
	"context"
	"log/slog"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/rule/connector"
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
	name     string
	config   MqttConfig
	conn     *connector.Mqtt
	handlers sync.Map // like: map[string]MsgHander

	started bool
	status  rmodel.StatusInfo
}

func newMqtt(ctx context.Context, name string, cfg map[string]any, conn connector.Conn) (Source, error) {
	var ac MqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "decode config")
	}
	var mqttConn *connector.Mqtt
	if c, ok := conn.(*connector.Mqtt); !ok {
		return nil, errors.New("wrong connector type for mqtt source")
	} else {
		mqttConn = c
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
	slog.Info("Rule starting source", "type", TypeMqtt, "name", m.name)
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
	slog.Info("Rule stopping source", "type", TypeMqtt, "name", m.name)
	if !m.started {
		slog.Info("Rule skip stopping source (not started)", "type", TypeMqtt, "name", m.name)
		return
	}
	m.started = false
	m.status = rmodel.StatusNotStarted()
	err := m.conn.UnSubscribe(m.ctx, m.config.Topic)
	if err != nil {
		slog.Error("Rule source mqtt stop, failed to unsubscribe", "name", m.name, "error", err)
	} else {
		slog.Info("Rule stopped source", "type", TypeMqtt, "name", m.name)
	}
}

func (m *mqttImpl) Status() rmodel.StatusInfo {
	if !m.started {
		return rmodel.StatusNotStarted()
	}
	connSt := m.conn.Status()
	if connSt.Status == rmodel.Disconnected {
		return rmodel.StatusDisconnected("connector "+m.conn.Name()+" is disconnected: "+connSt.Reason, connSt.Error)
	}
	return m.status
}

func (m *mqttImpl) Name() string {
	return m.name
}

func (*mqttImpl) Type() string {
	return TypeMqtt
}

// OnMsg register MsgHandler which can't be blocked when it is called
func (m *mqttImpl) OnMsg(ruleName string, h MsgHander) {
	m.handlers.Store(ruleName, h)
	if h == nil {
		m.handlers.Delete(ruleName)
	}
}

func (m *mqttImpl) subscribe() error {
	return m.conn.Subscribe(m.ctx, m.config.Topic, m.config.Qos, func(cl mqtt.Client, msg mqtt.Message) {
		if !m.started {
			return
		}
		thId, err := model.GetThingIdFromTopic(msg.Topic())
		if err != nil {
			slog.Error("Can't get thing id from topic in embed mqtt broker subscription", "error", err)
		}
		mm := Msg{
			ThingId: thId,
			Topic:   msg.Topic(),
			Payload: string(msg.Payload()),
		}
		m.handlers.Range(func(key, value any) bool {
			value.(MsgHander)(mm)
			return true
		})

	})
}
