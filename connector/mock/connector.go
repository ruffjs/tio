package mock

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"ruff.io/tio/connector"
)

type PublishedMessage struct {
	Topic   string
	Payload []byte
	Method  string
}

type subscription struct {
	topic    string
	queue    string
	callback func(msg connector.Message)
	ctx      context.Context
}

type MockConnector struct {
	mu      sync.Mutex
	started bool
	ctx     context.Context
	cancel  context.CancelFunc

	Published []PublishedMessage

	subscriptions []subscription

	connectedThings map[string]bool
	presenceHandler connector.PresenceHandler

	queueCounters map[string]*atomic.Uint64
}

func NewMockConnector() *MockConnector {
	return &MockConnector{
		connectedThings: make(map[string]bool),
		queueCounters:   make(map[string]*atomic.Uint64),
	}
}

func (m *MockConnector) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	m.ctx, m.cancel = context.WithCancel(ctx)
	return nil
}

func (m *MockConnector) Publish(topic string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Published = append(m.Published, PublishedMessage{Topic: topic, Payload: payload, Method: "Publish"})
	return nil
}

func (m *MockConnector) PublishReliable(topic string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Published = append(m.Published, PublishedMessage{Topic: topic, Payload: payload, Method: "PublishReliable"})
	return nil
}

func (m *MockConnector) PublishRetained(topic string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Published = append(m.Published, PublishedMessage{Topic: topic, Payload: payload, Method: "PublishRetained"})
	return nil
}

func (m *MockConnector) PublishedMessages() []PublishedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	messages := make([]PublishedMessage, len(m.Published))
	copy(messages, m.Published)
	return messages
}

func (m *MockConnector) Subscribe(ctx context.Context, topic string, callback func(msg connector.Message)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions = append(m.subscriptions, subscription{
		topic:    topic,
		callback: callback,
		ctx:      ctx,
	})
	return nil
}

func (m *MockConnector) QueueSubscribe(ctx context.Context, topic, queue string, callback func(msg connector.Message)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions = append(m.subscriptions, subscription{
		topic:    topic,
		queue:    queue,
		callback: callback,
		ctx:      ctx,
	})
	if _, ok := m.queueCounters[queue]; !ok {
		m.queueCounters[queue] = &atomic.Uint64{}
	}
	return nil
}

func (m *MockConnector) Close(thingId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectedThings[thingId] = false
	return nil
}

func (m *MockConnector) Remove(thingId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connectedThings, thingId)
	return nil
}

func (m *MockConnector) IsConnected(thingId string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connectedThings[thingId], nil
}

func (m *MockConnector) OnLocalPresence(handler connector.PresenceHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.presenceHandler = handler
}

func (m *MockConnector) ClientInfo(thingId string) (connector.ClientInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	connected := m.connectedThings[thingId]
	return connector.ClientInfo{
		ClientId:  thingId,
		Username:  thingId,
		Connected: connected,
	}, nil
}

func (m *MockConnector) AllClientInfo() ([]connector.ClientInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []connector.ClientInfo
	for id, connected := range m.connectedThings {
		result = append(result, connector.ClientInfo{
			ClientId:  id,
			Username:  id,
			Connected: connected,
		})
	}
	return result, nil
}

func (m *MockConnector) SimulateMessage(topic string, payload []byte) {
	m.mu.Lock()
	subs := make([]subscription, len(m.subscriptions))
	copy(subs, m.subscriptions)

	queueSubs := make(map[string][]int)
	for i, s := range subs {
		if s.queue != "" {
			queueSubs[s.queue] = append(queueSubs[s.queue], i)
		}
	}
	m.mu.Unlock()

	msg := NewMessage(topic, payload)

	delivered := make(map[int]bool)

	for queue, indices := range queueSubs {
		var matching []int
		for _, idx := range indices {
			s := subs[idx]
			if s.ctx != nil && s.ctx.Err() != nil {
				continue
			}
			if matchTopic(s.topic, topic) {
				matching = append(matching, idx)
			}
		}
		if len(matching) > 0 {
			m.mu.Lock()
			counter := m.queueCounters[queue]
			m.mu.Unlock()
			pick := counter.Add(1) - 1
			chosen := matching[int(pick)%len(matching)]
			subs[chosen].callback(msg)
			delivered[chosen] = true
		}
	}

	for i, s := range subs {
		if delivered[i] {
			continue
		}
		if s.queue != "" {
			continue
		}
		if s.ctx != nil && s.ctx.Err() != nil {
			continue
		}
		if matchTopic(s.topic, topic) {
			s.callback(msg)
		}
	}
}

func (m *MockConnector) SimulatePresence(ci connector.ClientInfo) {
	if m.presenceHandler != nil {
		m.presenceHandler(ci)
	}
}

func (m *MockConnector) SetConnected(thingId string, connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectedThings[thingId] = connected
}

func (m *MockConnector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Published = nil
	m.subscriptions = nil
	m.connectedThings = make(map[string]bool)
	m.queueCounters = make(map[string]*atomic.Uint64)
}

func matchTopic(filter, topic string) bool {
	if filter == topic {
		return true
	}
	return matchLevels(strings.Split(filter, "/"), strings.Split(topic, "/"))
}

func matchLevels(filter, topic []string) bool {
	if len(filter) == 0 {
		return len(topic) == 0
	}
	if len(topic) == 0 {
		return filter[0] == "#"
	}
	if filter[0] == "#" {
		return true
	}
	if filter[0] == "+" || filter[0] == topic[0] {
		return matchLevels(filter[1:], topic[1:])
	}
	return false
}

var _ interface {
	Publish(string, []byte) error
	PublishReliable(string, []byte) error
	PublishRetained(string, []byte) error
	Subscribe(context.Context, string, func(msg connector.Message)) error
	QueueSubscribe(context.Context, string, string, func(msg connector.Message)) error
	Start(context.Context) error
	Close(string) error
	Remove(string) error
	IsConnected(string) (bool, error)
	OnLocalPresence(connector.PresenceHandler)
	ClientInfo(string) (connector.ClientInfo, error)
	AllClientInfo() ([]connector.ClientInfo, error)
} = (*MockConnector)(nil)

var _ connector.Message = (*MockMessage)(nil)
