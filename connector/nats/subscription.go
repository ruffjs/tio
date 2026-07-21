package nats

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

type Message interface {
	Topic() string
	Payload() []byte
}

type natsMessage struct {
	topic   string
	payload []byte
}

func (m *natsMessage) Topic() string   { return m.topic }
func (m *natsMessage) Payload() []byte { return m.payload }

func (c *Connector) Subscribe(ctx context.Context, topic string, callback func(msg Message)) error {
	return c.subscribe(ctx, topic, "", callback, false)
}

func (c *Connector) QueueSubscribe(ctx context.Context, topic, queue string, callback func(msg Message)) error {
	if queue == "" {
		return errors.New("queue group name must not be empty")
	}
	return c.subscribe(ctx, topic, queue, callback, true)
}

func (c *Connector) subscribe(ctx context.Context, topic, queue string, callback func(msg Message), isQueue bool) error {
	subjects, err := MqttSubscriptionToNatsSubjects(topic)
	if err != nil {
		return fmt.Errorf("convert topic %q: %w", topic, err)
	}

	subs := make([]*nats.Subscription, 0, len(subjects))
	defer func() {
		if err != nil {
			for _, s := range subs {
				_ = s.Unsubscribe()
			}
		}
	}()

	handler := func(msg *nats.Msg) {
		topic, convErr := NatsSubjectToMqttTopic(msg.Subject)
		if convErr != nil {
			return
		}
		callback(&natsMessage{topic: topic, payload: msg.Data})
	}

	for _, subj := range subjects {
		var sub *nats.Subscription
		if isQueue {
			sub, err = c.natsConn.QueueSubscribe(subj, queue, handler)
		} else {
			sub, err = c.natsConn.Subscribe(subj, handler)
		}
		if err != nil {
			return fmt.Errorf("subscribe %q: %w", subj, err)
		}
		subs = append(subs, sub)
	}

	if err = c.natsConn.Flush(); err != nil {
		return fmt.Errorf("flush subscriptions: %w", err)
	}

	go func() {
		<-ctx.Done()
		for _, s := range subs {
			_ = s.Unsubscribe()
		}
	}()

	return nil
}
