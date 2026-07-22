package mock

type MockMessage struct {
	topic   string
	payload []byte
}

func NewMessage(topic string, payload []byte) *MockMessage {
	return &MockMessage{topic: topic, payload: payload}
}

func (m *MockMessage) Topic() string   { return m.topic }
func (m *MockMessage) Payload() []byte { return m.payload }
