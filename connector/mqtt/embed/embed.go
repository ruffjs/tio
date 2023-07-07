package embed

// embedded mqtt broker adapter
import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"ruff.io/tio/shadow"
)

type embedMqttAdapter struct {
}

var _ shadow.Connectivity = (*embedMqttAdapter)(nil)

func NewEmbedAdapter() shadow.Connectivity {
	return &embedMqttAdapter{}
}

func (m *embedMqttAdapter) IsConnected(thingId string) (bool, error) {
	if BrokerInstance() == nil {
		return false, errors.New("mochi embed mqtt server is not initialized")
	}
	return BrokerInstance().IsConnected(thingId), nil
}

func (m *embedMqttAdapter) OnConnect() <-chan shadow.Event {
	return BrokerInstance().OnConnect()
}

func (m *embedMqttAdapter) ClientInfo(thingId string) (shadow.ClientInfo, error) {
	return BrokerInstance().ClientInfo(thingId)
}

func (m *embedMqttAdapter) AllClientInfo() ([]shadow.ClientInfo, error) {
	return BrokerInstance().AllClientInfo()
}

func (m *embedMqttAdapter) Start(ctx context.Context) error {
	return nil
}

func (m *embedMqttAdapter) Close(thingId string) error {
	ok := BrokerInstance().CloseClient(thingId)
	if !ok {
		return errors.New(fmt.Sprintf("mqtt borker close client failed, thingId=%s", thingId))
	}
	return nil
}

func (m *embedMqttAdapter) Remove(thingId string) error {
	_ = m.Close(thingId)
	go func() {
		// wait for thing connection closed
		time.Sleep(time.Second)
		_ = BrokerInstance().Publish(shadow.TopicPresence(thingId), nil, true, 0)
	}()
	return nil
}
