package source

import (
	"context"
	"log/slog"
	"runtime"
	"sync"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

const TypeEmbedMqtt = "embed-mqtt"

func init() {
	Register(TypeEmbedMqtt, newEmbedMqtt)
}

type EmbedMqttConfig struct {
	Topic string
}

type embedMqttImpl struct {
	ctx            context.Context
	name           string
	config         EmbedMqttConfig
	subscriptionId int
	handlers       sync.Map // like: map[string]MsgHander

	mu sync.RWMutex

	started bool
	status  model.StatusInfo
}

func newEmbedMqtt(ctx context.Context, name string, cfg map[string]any, conn connector.Conn) (Source, error) {
	var ac EmbedMqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "failed to decode source embed-mqtt config")
	}
	m := &embedMqttImpl{
		ctx:    ctx,
		name:   name,
		config: ac,
		status: model.StatusNotStarted(),
	}

	runtime.SetFinalizer(m, func(obj Source) {
		slog.Debug("Rule source is being garbage collected", "type", obj.Type(), "name", obj.Name())
	})

	return m, nil
}

func (m *embedMqttImpl) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil
	}
	m.started = true
	return m.sub()
}

func (m *embedMqttImpl) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscriptionId > 0 {
		err := embed.BrokerInstance().Unsubscribe(m.config.Topic, m.subscriptionId)
		if err != nil {
			slog.Error("Rule source embed-mqtt stop, failed to unsubscribe", "name", m.name, "error", err)
		}
	}
	m.started = false
	m.status = model.StatusNotStarted()
}

func (m *embedMqttImpl) Status() model.StatusInfo {
	return m.status
}

func (m *embedMqttImpl) Name() string {
	return m.name
}

func (*embedMqttImpl) Type() string {
	return TypeEmbedMqtt
}

// OnMsg register MsgHandler which can't be blocked when it is called
func (m *embedMqttImpl) OnMsg(ruleName string, h MsgHander) {
	m.handlers.Store(ruleName, h)
}

func (m *embedMqttImpl) sub() error {
	subId, err := embed.BrokerInstance().Subscribe(m.config.Topic, func(msg embed.Msg) {
		mm := Msg{
			ThingId: msg.ThingId,
			Topic:   msg.Topic,
			Payload: string(msg.Payload),
		}
		m.handlers.Range(func(key, value any) bool {
			value.(MsgHander)(mm)
			return true
		})
	})
	m.subscriptionId = subId
	if err != nil {
		m.status = model.StatusDisconnected(err.Error(), err)
		return errors.WithMessagef(err, "subscribe topic %q", m.config.Topic)
	} else {
		m.status = model.StatusConnected()
		return nil
	}
}
