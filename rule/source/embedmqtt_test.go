package source

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	connmock "ruff.io/tio/connector/mock"
	sinkpkg "ruff.io/tio/rule/sink"
)

func TestEmbedMqttSourceSink(t *testing.T) {
	mc := connmock.NewMockConnector()
	require.NoError(t, mc.Start(context.Background()))

	src, err := New(context.Background(), Config{
		Name:    "src1",
		Type:    "embed-mqtt",
		Enabled: true,
		Options: map[string]any{"topic": "#"},
	}, nil, mc)
	require.NoError(t, err)

	snk, err := sinkpkg.New(context.Background(), sinkpkg.Config{
		Name:    "sink1",
		Type:    "embed-mqtt",
		Enabled: true,
		Options: map[string]any{"topic": "out/${thingId}"},
	}, nil, mc)
	require.NoError(t, err)
	require.NoError(t, snk.Start())

	src.OnMsg("r1", func(msg Msg) {
		snk.Publish(sinkpkg.Msg{ThingId: msg.ThingId, Topic: msg.Topic, Payload: msg.Payload})
	})
	require.NoError(t, src.Start())

	cases := []struct {
		topic   string
		thingId string
	}{
		{"$iothub/things/dev1/property", "dev1"},
		{"$iothub/user/things/dev3/custom", "dev3"},
		{"tio/dev1/data", "dev1"},
		{"tio/dev1/things/dev2/x", "dev1"},
	}
	for _, tc := range cases {
		before := len(mc.PublishedMessages())
		mc.SimulateMessage(tc.topic, []byte(`{"ok":true}`))
		require.Eventually(t, func() bool { return len(mc.PublishedMessages()) > before }, time.Second, 10*time.Millisecond)
		msgs := mc.PublishedMessages()
		got := msgs[len(msgs)-1]
		require.Equal(t, "out/"+tc.thingId, got.Topic, "topic=%s", tc.topic)
	}

	src.Stop()
	require.NoError(t, snk.Stop())
}

func TestEmbedMqttSource_ThingIdParsing(t *testing.T) {
	mc := connmock.NewMockConnector()
	require.NoError(t, mc.Start(context.Background()))

	s, err := New(context.Background(), Config{
		Name:    "src1",
		Type:    "embed-mqtt",
		Enabled: true,
		Options: map[string]any{"topic": "#"},
	}, nil, mc)
	require.NoError(t, err)

	got := make(chan Msg, 8)
	s.OnMsg("r1", func(msg Msg) { got <- msg })
	require.NoError(t, s.Start())

	cases := []struct {
		topic   string
		thingId string
	}{
		{"tio/dev1/data", "dev1"},
		{"$iothub/things/dev2/property", "dev2"},
		{"$iothub/user/things/dev3/custom", "dev3"},
		{"tio/dev1/things/dev2/x", "dev1"},
	}
	for _, tc := range cases {
		mc.SimulateMessage(tc.topic, []byte(`{}`))
		select {
		case msg := <-got:
			require.Equal(t, tc.thingId, msg.ThingId, "topic=%s", tc.topic)
			require.Equal(t, tc.topic, msg.Topic)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for %s", tc.topic)
		}
	}

	s.Stop()
}
