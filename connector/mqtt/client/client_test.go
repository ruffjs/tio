package client

import (
	"context"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
)

func TestRemoveSubscriber(t *testing.T) {
	noop := func(mqtt.Client, mqtt.Message) {}

	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	ctxC, cancelC := context.WithCancel(context.Background())
	defer cancelC()

	newClient := func() *mqttClient {
		return &mqttClient{topics: map[string]*topicSub{
			"shared": {qos: 1, subscribed: true, handlers: []subscriber{
				{ctx: ctxA, callback: noop},
				{ctx: ctxB, callback: noop},
			}},
			"other": {qos: 0, subscribed: true, handlers: []subscriber{
				{ctx: ctxC, callback: noop},
			}},
		}}
	}

	t.Run("first of two same-topic handlers leaves one and keeps subscription", func(t *testing.T) {
		c := newClient()
		remaining, unsubscribe := c.removeSubscriber("shared", ctxA)
		require.Equal(t, 1, remaining)
		require.False(t, unsubscribe)
		require.Len(t, c.topics["shared"].handlers, 1)
		require.True(t, c.topics["shared"].subscribed)
		require.Equal(t, ctxB, c.topics["shared"].handlers[0].ctx)
	})

	t.Run("last same-topic handler deletes topic and unsubscribes", func(t *testing.T) {
		c := newClient()
		_, _ = c.removeSubscriber("shared", ctxA)
		remaining, unsubscribe := c.removeSubscriber("shared", ctxB)
		require.Equal(t, 0, remaining)
		require.True(t, unsubscribe)
		_, ok := c.topics["shared"]
		require.False(t, ok)
	})

	t.Run("different topics are independent", func(t *testing.T) {
		c := newClient()
		_, _ = c.removeSubscriber("shared", ctxA)
		remaining, unsubscribe := c.removeSubscriber("other", ctxC)
		require.Equal(t, 0, remaining)
		require.True(t, unsubscribe)
		require.Contains(t, c.topics, "shared")
		require.NotContains(t, c.topics, "other")
	})

	t.Run("unknown topic is a no-op", func(t *testing.T) {
		c := newClient()
		remaining, unsubscribe := c.removeSubscriber("missing", ctxA)
		require.Equal(t, 0, remaining)
		require.False(t, unsubscribe)
	})
}
