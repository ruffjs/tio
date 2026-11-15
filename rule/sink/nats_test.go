package sink

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector/mock"
)

func TestNatsSinkRestartKeepsPublishing(t *testing.T) {
	mc := mock.NewMockConnector()
	require.NoError(t, mc.Start(context.Background()))
	s, err := NewNats(context.Background(), "s1", map[string]any{"topic": "out/${thingId}"}, nil, mc)
	require.NoError(t, err)

	delivered := func() []mock.PublishedMessage {
		return mc.PublishedMessages()
	}

	require.NoError(t, s.Start())
	s.Publish(Msg{ThingId: "t1", Payload: "one"})
	require.Eventually(t, func() bool { return len(delivered()) == 1 }, time.Second, 10*time.Millisecond)

	require.NoError(t, s.Stop())
	s.Publish(Msg{ThingId: "t1", Payload: "dropped"})
	time.Sleep(50 * time.Millisecond)
	require.Len(t, delivered(), 1)

	require.NoError(t, s.Start())
	s.Publish(Msg{ThingId: "t1", Payload: "two"})
	require.Eventually(t, func() bool {
		msgs := delivered()
		return len(msgs) == 2 && string(msgs[1].Payload) == "two"
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, s.Stop())
}
