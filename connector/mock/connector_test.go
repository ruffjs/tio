package mock

import (
	"context"
	"sync"
	"testing"

	"ruff.io/tio/connector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishRecordsMessage(t *testing.T) {
	c := NewMockConnector()
	require.NoError(t, c.Publish("t/1", []byte("hello")))
	require.NoError(t, c.PublishReliable("t/2", []byte("reliable")))
	require.NoError(t, c.PublishRetained("t/3", []byte("retained")))

	require.Len(t, c.Published, 3)
	assert.Equal(t, "t/1", c.Published[0].Topic)
	assert.Equal(t, "Publish", c.Published[0].Method)
	assert.Equal(t, "PublishReliable", c.Published[1].Method)
	assert.Equal(t, "PublishRetained", c.Published[2].Method)
	assert.Equal(t, []byte("hello"), c.Published[0].Payload)
}

func TestSubscribeReceivesSimulatedMessages(t *testing.T) {
	c := NewMockConnector()
	var received []connector.Message
	var mu sync.Mutex

	err := c.Subscribe(context.Background(), "foo/bar", func(msg connector.Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})
	require.NoError(t, err)

	c.SimulateMessage("foo/bar", []byte("data"))

	mu.Lock()
	require.Len(t, received, 1)
	assert.Equal(t, "foo/bar", received[0].Topic())
	assert.Equal(t, []byte("data"), received[0].Payload())
	mu.Unlock()
}

func TestWildcardPlusSubscribe(t *testing.T) {
	c := NewMockConnector()
	var received []connector.Message
	var mu sync.Mutex

	_ = c.Subscribe(context.Background(), "foo/+/bar", func(msg connector.Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})

	c.SimulateMessage("foo/x/bar", []byte("match"))
	c.SimulateMessage("foo/y/bar", []byte("match2"))
	c.SimulateMessage("foo/x/y/bar", []byte("nomatch"))

	mu.Lock()
	require.Len(t, received, 2)
	mu.Unlock()
}

func TestHashWildcard(t *testing.T) {
	c := NewMockConnector()
	var received []connector.Message
	var mu sync.Mutex

	_ = c.Subscribe(context.Background(), "foo/#", func(msg connector.Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})

	c.SimulateMessage("foo/bar/baz", []byte("deep"))
	c.SimulateMessage("foo", []byte("root"))
	c.SimulateMessage("bar/foo", []byte("nomatch"))

	mu.Lock()
	require.Len(t, received, 2)
	mu.Unlock()
}

func TestQueueSubscribeDistributes(t *testing.T) {
	c := NewMockConnector()
	var count1, count2 int
	var mu sync.Mutex

	_ = c.QueueSubscribe(context.Background(), "q/topic", "group1", func(msg connector.Message) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	_ = c.QueueSubscribe(context.Background(), "q/topic", "group1", func(msg connector.Message) {
		mu.Lock()
		count2++
		mu.Unlock()
	})

	for i := 0; i < 10; i++ {
		c.SimulateMessage("q/topic", []byte("msg"))
	}

	mu.Lock()
	assert.Equal(t, 10, count1+count2, "total received should be 10")
	assert.True(t, count1 > 0, "subscriber 1 should receive some messages")
	assert.True(t, count2 > 0, "subscriber 2 should receive some messages")
	mu.Unlock()
}

func TestOnLocalPresenceCallback(t *testing.T) {
	c := NewMockConnector()

	var received []connector.ClientInfo
	var mu sync.Mutex

	c.OnLocalPresence(func(ci connector.ClientInfo) {
		mu.Lock()
		received = append(received, ci)
		mu.Unlock()
	})

	ci := connector.ClientInfo{
		ClientId:  "client1",
		Username:  "thing1",
		Connected: true,
	}
	c.SimulatePresence(ci)

	mu.Lock()
	require.Len(t, received, 1)
	assert.Equal(t, "thing1", received[0].Username)
	assert.True(t, received[0].Connected)
	mu.Unlock()
}

func TestContextCancellation(t *testing.T) {
	c := NewMockConnector()
	var received int
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	_ = c.Subscribe(ctx, "cancel/topic", func(msg connector.Message) {
		mu.Lock()
		received++
		mu.Unlock()
	})

	c.SimulateMessage("cancel/topic", []byte("before"))
	mu.Lock()
	assert.Equal(t, 1, received)
	mu.Unlock()

	cancel()

	c.SimulateMessage("cancel/topic", []byte("after"))
	mu.Lock()
	assert.Equal(t, 1, received, "should not receive after context cancel")
	mu.Unlock()
}

func TestSetConnectedAndIsConnected(t *testing.T) {
	c := NewMockConnector()

	c.SetConnected("thing1", true)
	ok, err := c.IsConnected("thing1")
	require.NoError(t, err)
	assert.True(t, ok)

	_ = c.Close("thing1")
	ok, err = c.IsConnected("thing1")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRemove(t *testing.T) {
	c := NewMockConnector()

	c.SetConnected("thing1", true)
	_ = c.Remove("thing1")

	ok, err := c.IsConnected("thing1")
	require.NoError(t, err)
	assert.False(t, ok)

	infos, err := c.AllClientInfo()
	require.NoError(t, err)
	assert.Empty(t, infos)
}

func TestReset(t *testing.T) {
	c := NewMockConnector()

	_ = c.Publish("t", []byte("data"))
	c.SetConnected("thing1", true)
	_ = c.Subscribe(context.Background(), "t", func(msg connector.Message) {})

	c.Reset()

	assert.Empty(t, c.Published)
	infos, _ := c.AllClientInfo()
	assert.Empty(t, infos)
}

func TestMatchTopic(t *testing.T) {
	cases := []struct {
		filter string
		topic  string
		want   bool
	}{
		{"foo/bar", "foo/bar", true},
		{"foo/bar", "foo/baz", false},
		{"foo/+/bar", "foo/x/bar", true},
		{"foo/+/bar", "foo/x/y/bar", false},
		{"foo/#", "foo", true},
		{"foo/#", "foo/bar", true},
		{"foo/#", "foo/bar/baz", true},
		{"#", "anything", true},
		{"+/bar", "foo/bar", true},
		{"+/bar", "foo/baz", false},
	}

	for _, tc := range cases {
		t.Run(tc.filter+"_"+tc.topic, func(t *testing.T) {
			assert.Equal(t, tc.want, matchTopic(tc.filter, tc.topic))
		})
	}
}
