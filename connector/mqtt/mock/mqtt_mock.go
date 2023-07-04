package mock

import (
	"context"
	"math/rand"
	"time"

	mq "ruff.io/tio/connector/mqtt/client"

	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/mock"
)

// mock mqtt client

type SubCallback func(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler)
type PubCallback func(topic string, qos byte, retained bool, payload interface{})

type MockedMqttClient struct {
	mq.Client
	mock.Mock
	subscribers map[string]mqtt.MessageHandler // topic => message handler

	ClientId          string
	PublishCallback   PubCallback
	SubscribeCallback SubCallback
}

var _ mq.Client = (*MockedMqttClient)(nil)

func NewMqttClient(clientId string, pc PubCallback, sc SubCallback) *MockedMqttClient {
	return &MockedMqttClient{
		subscribers:       make(map[string]mqtt.MessageHandler),
		PublishCallback:   pc,
		SubscribeCallback: sc,
	}
}

type Token struct {
	DoneCh chan struct{}
}

func NewMockToken() *Token {
	d := make(chan struct{})
	close(d)
	return &Token{DoneCh: d}
}

func (m *MockedMqttClient) route(topic string, payload interface{}) {
	for t, c := range m.subscribers {
		if MatchTopic(t, topic) {
			c(nil, MqttMsg{TopicName: topic, PayloadData: payload.([]byte)})
		}
	}
}

func (m *MockedMqttClient) Connect(ctx context.Context) error {
	return nil
}

func (m *MockedMqttClient) OnConnect(f func()) {
	println("OnConnect, ignore callback function")
}

func (m *MockedMqttClient) Subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	args := m.Called(ctx, topic, qos, callback)
	go func() {
		if m.SubscribeCallback != nil {
			m.SubscribeCallback(ctx, topic, qos, callback)
		}
	}()
	m.subscribers[topic] = callback
	log.Debugf("Subscribe mock: topic=%q qos=%v", topic, qos)
	if args.Get(0) == nil {
		return nil
	} else {
		return args.Get(0).(error)
	}
}

func (m *MockedMqttClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	args := m.Called(topic, qos, retained, payload)
	if m.PublishCallback != nil {
		m.PublishCallback(topic, qos, retained, payload)
	}
	log.Debugf("Publish mock: topic=%q qos=%v retained=%v payload=%s", topic, qos, retained, payload)
	m.route(topic, payload)
	return args.Get(0).(mqtt.Token)
}

func (m *MockedMqttClient) IsConnected(thingId string) (bool, error) {
	args := m.Called(thingId)
	res := args.Get(0).(bool)
	var errRes error
	if args.Get(1) != nil {
		errRes = args.Get(1).(error)
	}
	return res, errRes
}

func (s *Token) Wait() bool {
	return true
}

func (s *Token) WaitTimeout(duration time.Duration) bool {
	return true
}

func (s *Token) Done() <-chan struct{} {
	return s.DoneCh
}

func (s *Token) Error() error {
	return nil
}

// mock mqtt Message

type MqttMsg struct {
	TopicName   string
	PayloadData []byte
}

var _ mqtt.Message = (*MqttMsg)(nil)

func (m MqttMsg) Duplicate() bool {
	return false
}

func (m MqttMsg) Qos() byte {
	return 1
}

func (m MqttMsg) Retained() bool {
	return false
}

func (m MqttMsg) Topic() string {
	return m.TopicName
}

func (m MqttMsg) MessageID() uint16 {
	return uint16(rand.Uint32())
}

func (m MqttMsg) Payload() []byte {
	return m.PayloadData
}

func (m MqttMsg) Ack() {
}

// AdapterImpl mock adapter
type AdapterImpl struct {
	mock.Mock
	shadow.ConnectChecker
}

var _ shadow.ConnectChecker = (*AdapterImpl)(nil)

func (m *AdapterImpl) OnConnect() <-chan shadow.Event {
	args := m.Called()
	res := args.Get(0).(<-chan shadow.Event)
	return res
}

func (m *AdapterImpl) IsConnected(thingId string) (bool, error) {
	args := m.Called(thingId)
	res := args.Get(0).(bool)
	var errRes error
	if args.Get(1) != nil {
		errRes = args.Get(1).(error)
	}
	return res, errRes
}

func (m *AdapterImpl) ClientInfo(thingId string) (shadow.ClientInfo, error) {
	return shadow.ClientInfo{ClientId: thingId}, nil
}
