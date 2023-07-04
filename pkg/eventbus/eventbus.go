package eventbus

import (
	"sync"
	"time"

	"ruff.io/tio/pkg/log"
)

type EventBus[T any] struct {
	subscribers map[string][]chan T
	mutex       sync.RWMutex
}

func NewEventBus[T any]() *EventBus[T] {
	return &EventBus[T]{
		subscribers: make(map[string][]chan T),
	}
}

func (eb *EventBus[T]) Subscribe(event string) <-chan T {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	ch := make(chan T)
	eb.subscribers[event] = append(eb.subscribers[event], ch)

	return ch
}

func (eb *EventBus[T]) Unsubscribe(event string, ch <-chan T) {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()
	if subs, ok := eb.subscribers[event]; ok {
		for i, s := range subs {
			if s == ch {
				res := append(subs[:i], subs[i+1:]...)
				eb.subscribers[event] = res
				break
			}
		}
	}
}

func (eb *EventBus[T]) Publish(event string, message T) {
	eb.mutex.RLock()
	defer eb.mutex.RUnlock()

	if subscribers, ok := eb.subscribers[event]; ok {
		for _, ch := range subscribers {
			select {
			case ch <- message:
			case <-time.After(time.Millisecond):
				log.Error("EventBus notify event timeout in 1 ms")
			}
		}
	}
}
