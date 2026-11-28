package sink

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector/mock"
)

func TestEmbedMqttSink_Publish(t *testing.T) {
	mc := mock.NewMockConnector()
	require.NoError(t, mc.Start(context.Background()))

	s, err := New(context.Background(), Config{
		Name:    "sink1",
		Type:    "embed-mqtt",
		Enabled: true,
		Options: map[string]any{"topic": "tio/${thingId}/x", "qos": 0, "retained": false},
	}, nil, mc)
	require.NoError(t, err)
	require.NoError(t, s.Start())

	s.Publish(Msg{ThingId: "dev1", Payload: `{"ok":true}`})
	require.Eventually(t, func() bool {
		msgs := mc.PublishedMessages()
		return len(msgs) == 1 && msgs[0].Topic == "tio/dev1/x"
	}, time.Second, 10*time.Millisecond)

	msgs := mc.PublishedMessages()
	require.Equal(t, []byte(`{"ok":true}`), msgs[0].Payload)
	require.NoError(t, s.Stop())
}

func TestEmbedMqttSink_MissingThingId(t *testing.T) {
	mc := mock.NewMockConnector()
	require.NoError(t, mc.Start(context.Background()))

	s, err := New(context.Background(), Config{
		Name:    "sink1",
		Type:    "embed-mqtt",
		Enabled: true,
		Options: map[string]any{"topic": "tio/${thingId}/x"},
	}, nil, mc)
	require.NoError(t, err)
	require.NoError(t, s.Start())

	s.Publish(Msg{ThingId: "", Payload: "bad"})
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, mc.PublishedMessages())
	require.NoError(t, s.Stop())
}
