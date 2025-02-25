package sink

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

const TypeEmbedMqtt = "embed-mqtt"

func init() {
	Register(TypeEmbedMqtt, NewEmbedMqtt)
}

type EmbedMqttConfig struct {
	Topic    string `json:"topic"`
	Qos      byte   `json:"qos"`
	Retained bool   `json:"retained"`
}

func NewEmbedMqtt(ctx context.Context, name string, cfg map[string]any, _ connector.Conn) (Sink, error) {
	var ac EmbedMqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	m := &embedMqttImpl{
		ctx:    ctx,
		name:   name,
		config: ac,
	}

	runtime.SetFinalizer(m, func(obj Sink) {
		slog.Debug("Rule sink is being garbage collected", "type", obj.Type(), "name", obj.Name())
	})

	return m, nil
}

type embedMqttImpl struct {
	ctx    context.Context
	name   string
	config EmbedMqttConfig

	mu      sync.RWMutex
	started bool
	metric  Metric
}

func (m *embedMqttImpl) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = false
	slog.Info("Rule stop sinked", "type", m.Type(), "name", m.name)
	return nil
}

func (m *embedMqttImpl) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	slog.Info("Rule started sink", "type", m.Type(), "name", m.name)
	return m.Status().Error
}

func (m *embedMqttImpl) Status() model.StatusInfo {
	if !m.started {
		return model.StatusNotStarted()
	}
	// embed mqtt always on
	s := model.StatusConnected()
	s.Metric = Metric{
		Received: atomic.LoadInt64(&m.metric.Received),
		Failed:   atomic.LoadInt64(&m.metric.Failed),
		Success:  atomic.LoadInt64(&m.metric.Success),
	}
	return s
}

func (m *embedMqttImpl) Publish(msg Msg) {
	var err error
	if !m.started {
		return
	}

	defer func() {
		atomic.AddInt64(&m.metric.Received, 1)
		if err != nil {
			atomic.AddInt64(&m.metric.Failed, 1)
		} else {
			atomic.AddInt64(&m.metric.Success, 1)
		}
	}()

	topic := m.config.Topic
	if strings.Contains(m.config.Topic, "${thingId}") {
		topic = strings.ReplaceAll(m.config.Topic, "${thingId}", msg.ThingId)
		if msg.ThingId == "" {
			slog.Error("Rule sink embed-mqtt publish mesage", "name", m.name, "error", "missing thingId", "message", msg)
			err = fmt.Errorf("missing thingId")
			return
		}
	}
	err = embed.BrokerInstance().Publish(topic, []byte(msg.Payload), m.config.Retained, m.config.Qos)
	if err != nil {
		slog.Error("Rule sink embed-mqtt publish mesage", "name", m.name, "error", err, "message", msg)
	}
}

func (m *embedMqttImpl) Name() string {
	return m.name
}

func (*embedMqttImpl) Type() string {
	return TypeEmbedMqtt
}
