package embed

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageQueueHook_OnPublish(t *testing.T) {
	server := mqtt.New(nil)
	hook, err := newMessageQueueHook(server, messageQueueConfig{StorageType: "memory"})
	require.NoError(t, err)

	queueName := "test-q"
	hook.queueRegistry.Store(queueName, "topic/+")

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  1,
		},
		TopicName: "topic/1",
		Payload:   []byte("hello"),
	}

	_, err = hook.OnPublish(nil, pk)
	require.NoError(t, err)

	msg := hook.storage.Dequeue(context.Background(), queueName)
	require.NotNil(t, msg)
	assert.Equal(t, "topic/1", msg.Topic)
	assert.Equal(t, []byte("hello"), msg.Payload)
}

func TestMessageQueueHook_WsHandler(t *testing.T) {
	server := mqtt.New(nil)
	hook, err := newMessageQueueHook(server, messageQueueConfig{StorageType: "memory"})
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(hook.WsHandler))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	q := u.Query()
	q.Set("queue", "test-q")
	u.RawQuery = q.Encode()

	// 1. Prepare messages in storage
	msg1 := &queuedMessage{MsgID: "m1", Topic: "t1", Payload: []byte("p1")}
	hook.storage.Enqueue(context.Background(), "test-q", msg1)

	// 2. Connect via WS
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer ws.Close()

	// 3. Send pull request
	req := wsRequest{Action: "pull", Count: 1}
	err = ws.WriteJSON(req)
	require.NoError(t, err)

	// 4. Receive message
	var received queuedMessage
	err = ws.ReadJSON(&received)
	require.NoError(t, err)
	assert.Equal(t, "m1", received.MsgID)
}

func TestMessageQueueHook_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	server := mqtt.New(nil)
	hook, err := newMessageQueueHook(server, messageQueueConfig{StorageType: "memory"})
	require.NoError(t, err)

	queueName := "stress/+"
	hook.queueRegistry.Store(queueName, "stress/+")

	ts := httptest.NewServer(http.HandlerFunc(hook.WsHandler))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	q := u.Query()
	q.Set("queue", queueName)
	u.RawQuery = q.Encode()

	expectedTotal := rand.Int64N(10_000)
	var totalReceived atomic.Int64
	var wgConsume sync.WaitGroup

	// Publishers
	go func() {
		for i := range expectedTotal {
			pk := packets.Packet{
				FixedHeader: packets.FixedHeader{Type: packets.Publish},
				TopicName:   fmt.Sprintf("stress/%d", i%10),
				Payload:     []byte(fmt.Sprintf("payload-%d", i)),
			}
			_, _ = hook.OnPublish(nil, pk)
		}
	}()

	// Consumers
	for range 5 {
		wgConsume.Go(func() {
			for {
				if totalReceived.Load() >= expectedTotal {
					return
				}

				ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
				if err != nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}

				// Puller goroutine for this connection
				stopPull := make(chan struct{})
				go func() {
					ticker := time.NewTicker(20 * time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-stopPull:
							return
						case <-ticker.C:
							if totalReceived.Load() >= expectedTotal {
								return
							}
							_ = ws.WriteJSON(wsRequest{Action: "pull", Count: 500})
						}
					}
				}()

				// Reader loop
				for {
					if totalReceived.Load() >= expectedTotal {
						break
					}

					// Use a long deadline to avoid frequent timeouts
					_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
					var msg queuedMessage
					if err := ws.ReadJSON(&msg); err != nil {
						break // Connection error or timeout
					}
					totalReceived.Add(1)
				}

				close(stopPull)
				_ = ws.Close()

				if totalReceived.Load() >= expectedTotal {
					return
				}
			}
		})
	}

	// Wait for consumers with timeout
	done := make(chan struct{})
	go func() {
		wgConsume.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Errorf("Stress test timed out: %d/%d", totalReceived.Load(), expectedTotal)
	}

	assert.Equal(t, expectedTotal, totalReceived.Load())
}
