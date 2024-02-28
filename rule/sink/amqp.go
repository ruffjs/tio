package sink

import (
	"context"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"ruff.io/tio/rule/connector"
)

// AMQP sink for message forward

type AmqpConfig struct {
	Exchange   string `json:"exchange"`
	RoutingKey string `json:"routingKey"`
	// WaitAck        bool          `json:"waitAck"`
	// WaitAckTimeout time.Duration `json:"waitAckTimeout"`
}

func NewAmqp(name string, cfg AmqpConfig, conn *connector.Amqp) Sink {
	a := &amqpImpl{
		name:   name,
		config: cfg,
		conn:   conn,
		ch:     make(chan *Msg, 10000),
	}
	a.setup()
	go a.publishLoop()
	return a
}

type amqpImpl struct {
	name    string
	config  AmqpConfig
	ch      chan *Msg
	conn    *connector.Amqp
	channel *amqp.Channel
}

func (*amqpImpl) Type() string {
	return TypeAMQP
}

func (a *amqpImpl) Publish(msg Msg) {
	a.ch <- &msg
}

func (a *amqpImpl) publishLoop() {
	for {
		// wait connect
		for {
			if a.channel == nil || a.channel.IsClosed() {
				a.setup()
			} else {
				break
			}
			time.Sleep(time.Second)
		}

		msg := <-a.ch

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := a.channel.PublishWithContext(ctx,
			a.config.Exchange,
			a.config.RoutingKey,
			false,
			false,
			amqp.Publishing{
				Headers: amqp.Table{
					"thingId": msg.ThingId,
					"topic":   msg.Topic,
				},
				Timestamp: time.Now(),
				Body:      msg.Payload,
			},
		)
		if err != nil {
			slog.Error("Amqp publish error", "name", a.name, "error", err)
		} else {
			slog.Debug("Amqp published message", "name", a.name, "thingId", msg.ThingId)
		}
	}
}

func (a *amqpImpl) setup() error {
	if a.conn.Conn.IsClosed() {
		if err := a.conn.Setup(); err != nil {
			return err
		}
	}
	ch, err := a.conn.Conn.Channel()
	if err != nil {
		return err
	}
	a.channel = ch
	return nil
}
