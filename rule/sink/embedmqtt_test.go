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
	require.Eventually(t, func() bool { return len(mc.PublishedMessages()) == 1 }, time.Second, 10*time.Millisecond)

	msgs := mc.PublishedMessages()
	require.Equal(t, "tio/dev1/x", msgs[0].Topic)
	require.Equal(t, []byte(`{"ok":true}`), msgs[0].Payload)
	require.Equal(t, "Publish", msgs[0].Method)
	require.NoError(t, s.Stop())
}

func TestEmbedMqttSink_QosRetainedMethod(t *testing.T) {
	cases := []struct {
		name   string
		opts   map[string]any
		method string
	}{
		{"qos0", map[string]any{"topic": "t/${thingId}", "qos": 0, "retained": false}, "Publish"},
		{"qos1", map[string]any{"topic": "t/${thingId}", "qos": 1, "retained": false}, "PublishReliable"},
		{"retained", map[string]any{"topic": "t/${thingId}", "qos": 0, "retained": true}, "PublishRetained"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mc := mock.NewMockConnector()
			require.NoError(t, mc.Start(context.Background()))
			s, err := New(context.Background(), Config{Name: "s", Type: "embed-mqtt", Options: tc.opts}, nil, mc)
			require.NoError(t, err)
			require.NoError(t, s.Start())
			s.Publish(Msg{ThingId: "dev1", Payload: "p"})
			require.Eventually(t, func() bool { return len(mc.PublishedMessages()) == 1 }, time.Second, 10*time.Millisecond)
			require.Equal(t, tc.method, mc.PublishedMessages()[0].Method)
			require.NoError(t, s.Stop())
		})
	}
}

func TestEmbedMqttSink_RejectsQos2(t *testing.T) {
	mc := mock.NewMockConnector()
	_, err := New(context.Background(), Config{
		Name: "s", Type: "embed-mqtt", Options: map[string]any{"topic": "t", "qos": 2},
	}, nil, mc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "qos must be 0 or 1")
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
