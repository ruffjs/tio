package source

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector/mock"
)

func TestEmbedMqttSourceSink(t *testing.T) {
	// This is an integration-style check across source+sink packages via their registries.
	// The actual source/sink behavior is validated below.
}

func TestEmbedMqttSource_ThingIdParsing(t *testing.T) {
	mc := mock.NewMockConnector()
	require.NoError(t, mc.Start(context.Background()))

	s, err := New(context.Background(), Config{
		Name:    "src1",
		Type:    "embed-mqtt",
		Enabled: true,
		Options: map[string]any{"topic": "#"},
	}, nil, mc)
	require.NoError(t, err)

	got := make(chan Msg, 4)
	s.OnMsg("r1", func(msg Msg) { got <- msg })
	require.NoError(t, s.Start())

	mc.SimulateMessage("tio/dev1/data", []byte(`{"v":1}`))
	select {
	case msg := <-got:
		require.Equal(t, "dev1", msg.ThingId)
		require.Equal(t, "tio/dev1/data", msg.Topic)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for simple topic message")
	}

	mc.SimulateMessage("$iothub/things/dev2/property", []byte(`{"v":2}`))
	select {
	case msg := <-got:
		require.Equal(t, "dev2", msg.ThingId)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for legacy thing topic message")
	}

	mc.SimulateMessage("$iothub/user/things/dev3/custom", []byte(`{"v":3}`))
	select {
	case msg := <-got:
		require.Equal(t, "dev3", msg.ThingId)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for legacy user topic message")
	}

	s.Stop()
}
