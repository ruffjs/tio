package nats

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBroadcastSubscribeBothReceive(t *testing.T) {
	c := newTestConnector(t)

	var countA, countB int32
	ctx := context.Background()
	if err := c.Subscribe(ctx, "$iothub/things/dev1/broadcast", func(msg Message) { atomic.AddInt32(&countA, 1) }); err != nil {
		t.Fatalf("Subscribe A: %v", err)
	}
	if err := c.Subscribe(ctx, "$iothub/things/dev1/broadcast", func(msg Message) { atomic.AddInt32(&countB, 1) }); err != nil {
		t.Fatalf("Subscribe B: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := c.Publish("$iothub/things/dev1/broadcast", []byte("hello")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt32(&countA) != 1 {
		t.Fatalf("subscriber A got %d, want 1", countA)
	}
	if atomic.LoadInt32(&countB) != 1 {
		t.Fatalf("subscriber B got %d, want 1", countB)
	}
}

func TestQueueSubscribeDistributesMessages(t *testing.T) {
	c := newTestConnector(t)

	var countA, countB int32
	ctx := context.Background()
	if err := c.QueueSubscribe(ctx, "$iothub/things/dev1/queue", "workers", func(msg Message) { atomic.AddInt32(&countA, 1) }); err != nil {
		t.Fatalf("QueueSubscribe A: %v", err)
	}
	if err := c.QueueSubscribe(ctx, "$iothub/things/dev1/queue", "workers", func(msg Message) { atomic.AddInt32(&countB, 1) }); err != nil {
		t.Fatalf("QueueSubscribe B: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	const n = 20
	for i := 0; i < n; i++ {
		if err := c.Publish("$iothub/things/dev1/queue", []byte("work")); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
	time.Sleep(1 * time.Second)

	total := atomic.LoadInt32(&countA) + atomic.LoadInt32(&countB)
	if total != n {
		t.Fatalf("total received = %d, want %d (A=%d B=%d)", total, n, countA, countB)
	}
}

func TestHashSubscriptionReceivesDescendants(t *testing.T) {
	c := newTestConnector(t)

	received := make(chan string, 10)
	ctx := context.Background()
	if err := c.Subscribe(ctx, "$iothub/things/dev1/foo/#", func(msg Message) { received <- msg.Topic() }); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := c.Publish("$iothub/things/dev1/foo", []byte("exact")); err != nil {
		t.Fatalf("Publish foo: %v", err)
	}
	if err := c.Publish("$iothub/things/dev1/foo/bar", []byte("desc")); err != nil {
		t.Fatalf("Publish foo/bar: %v", err)
	}
	if err := c.Publish("$iothub/things/dev1/foo/bar/baz", []byte("deep")); err != nil {
		t.Fatalf("Publish foo/bar/baz: %v", err)
	}

	want := map[string]bool{
		"$iothub/things/dev1/foo":         false,
		"$iothub/things/dev1/foo/bar":     false,
		"$iothub/things/dev1/foo/bar/baz": false,
	}
	timeout := time.After(3 * time.Second)
	for len(want) > 0 {
		select {
		case topic := <-received:
			if _, ok := want[topic]; ok {
				want[topic] = true
				delete(want, topic)
			}
		case <-timeout:
			t.Fatalf("timeout waiting for topics, still missing: %v", want)
		}
	}
}

func TestPublishRejectsWildcard(t *testing.T) {
	c := newTestConnector(t)
	if err := c.Publish("$iothub/things/+/data", []byte("x")); err == nil {
		t.Fatal("expected error for wildcard publish topic")
	}
	if err := c.Publish("$iothub/things/#/data", []byte("x")); err == nil {
		t.Fatal("expected error for wildcard publish topic")
	}
}

func TestContextCancellationUnsubscribes(t *testing.T) {
	c := newTestConnector(t)

	ctx, cancel := context.WithCancel(context.Background())
	var received int32
	if err := c.Subscribe(ctx, "$iothub/things/dev1/cancelme", func(msg Message) { atomic.AddInt32(&received, 1) }); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := c.Publish("$iothub/things/dev1/cancelme", []byte("before")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if atomic.LoadInt32(&received) != 1 {
		t.Fatalf("before cancel: received = %d", received)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	if err := c.Publish("$iothub/things/dev1/cancelme", []byte("after")); err != nil {
		t.Fatalf("Publish after cancel: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&received) != 1 {
		t.Fatalf("after cancel: received = %d, want 1", received)
	}
}

func TestRepeatedSubscribeCancel(t *testing.T) {
	c := newTestConnector(t)

	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		if err := c.Subscribe(ctx, "$iothub/things/dev1/repeat", func(msg Message) {}); err != nil {
			t.Fatalf("Subscribe %d: %v", i, err)
		}
		cancel()
	}
	time.Sleep(200 * time.Millisecond)

	if err := c.Publish("$iothub/things/dev1/repeat", []byte("x")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
}

func TestQueueSubscribeEmptyQueueFails(t *testing.T) {
	c := newTestConnector(t)
	err := c.QueueSubscribe(context.Background(), "$iothub/things/dev1/x", "", func(msg Message) {})
	if err == nil {
		t.Fatal("expected error for empty queue group")
	}
}

func TestSubscribeInvalidTopicFails(t *testing.T) {
	c := newTestConnector(t)
	err := c.Subscribe(context.Background(), "$iothub/things/dev1/", func(msg Message) {})
	if err == nil {
		t.Fatal("expected error for trailing-slash topic")
	}
}

func TestConcurrentSubscribePublish(t *testing.T) {
	c := newTestConnector(t)

	const subs = 5
	const msgs = 30
	var totalReceived int32
	var wg sync.WaitGroup

	ctx := context.Background()
	for i := 0; i < subs; i++ {
		if err := c.Subscribe(ctx, "$iothub/things/dev1/stress/#", func(msg Message) { atomic.AddInt32(&totalReceived, 1) }); err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
	}
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < msgs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Publish("$iothub/things/dev1/stress/item", []byte("data"))
		}()
	}
	wg.Wait()

	time.Sleep(1 * time.Second)
	got := atomic.LoadInt32(&totalReceived)
	want := int32(subs * msgs)
	if got != want {
		t.Fatalf("total received = %d, want %d", got, want)
	}
}
