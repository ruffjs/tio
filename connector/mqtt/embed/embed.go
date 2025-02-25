package embed

// embedded mqtt broker adapter
import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"ruff.io/tio/connector"

	"github.com/pkg/errors"
)

type embedMqttAdapter struct {
}

var _ connector.Connectivity = (*embedMqttAdapter)(nil)

func NewEmbedAdapter() connector.Connectivity {
	return &embedMqttAdapter{}
}

func (m *embedMqttAdapter) IsConnected(thingId string) (bool, error) {
	if BrokerInstance() == nil {
		return false, errors.New("mochi embed mqtt server is not initialized")
	}
	return BrokerInstance().IsConnected(thingId), nil
}

func (m *embedMqttAdapter) OnConnect() <-chan connector.PresenceEvent {
	return BrokerInstance().OnConnect()
}

func (m *embedMqttAdapter) ClientInfo(thingId string) (connector.ClientInfo, error) {
	return BrokerInstance().ClientInfo(thingId)
}

func (m *embedMqttAdapter) AllClientInfo() ([]connector.ClientInfo, error) {
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
	err := m.Close(thingId)
	if err != nil {
		return err
	}
	go func() {
		// wait for thing connection closed
		time.Sleep(time.Second)
		err := BrokerInstance().Publish(connector.TopicPresence(thingId), nil, true, 0)
		if err != nil {
			slog.Error("Publish nil for removed client failed", "thingId", thingId)
		}
	}()
	return nil
}
