package source

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector/mock"
)

func TestMqttSourceRestartKeepsReceiving(t *testing.T) {
	mc := mock.NewMockConnector()
	require.NoError(t, mc.Start(context.Background()))
	s, err := newMqtt(context.Background(), "src1", map[string]any{"topic": "$iothub/things/+/up"}, nil, mc)
	require.NoError(t, err)

	var received atomic.Int64
	s.OnMsg("r1", func(msg Msg) { received.Add(1) })

	require.NoError(t, s.Start())
	mc.SimulateMessage("$iothub/things/t1/up", []byte("one"))
	require.Eventually(t, func() bool { return received.Load() == 1 }, time.Second, 10*time.Millisecond)

	s.Stop()
	mc.SimulateMessage("$iothub/things/t1/up", []byte("dropped"))
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int64(1), received.Load())

	require.NoError(t, s.Start())
	mc.SimulateMessage("$iothub/things/t1/up", []byte("two"))
	require.Eventually(t, func() bool { return received.Load() == 2 }, time.Second, 10*time.Millisecond)
	s.Stop()
}
