package sink

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mitchellh/mapstructure"
	amqp "github.com/rabbitmq/amqp091-go"
	"ruff.io/tio/rule/connector"
)

// AMQP sink for message forward

const TypeAMQP = "amqp"

func init() {
	Register(TypeAMQP, NewAmqp)
}

type AmqpConfig struct {
	Exchange   string `json:"exchange"`
	RoutingKey string `json:"routingKey"`
	// WaitAck        bool          `json:"waitAck"`
	// WaitAckTimeout time.Duration `json:"waitAckTimeout"`
}

func NewAmqp(name string, cfg map[string]any, conn connector.Conn) Sink {
	var ac AmqpConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("Rule sink AMQP decode config", "name", name, "error", err)
		os.Exit(1)
	}
	c, ok := conn.(*connector.Amqp)
	if !ok {
		slog.Error("Rule sink AMQP wrong connector")
		os.Exit(1)
	}

	a := &amqpImpl{
		name:   name,
		config: ac,
		conn:   c,
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

func (a *amqpImpl) Name() string {
	return a.name
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
				Body:      []byte(msg.Payload),
			},
		)
		if err != nil {
			slog.Error("Rule sink AMQP publish error", "name", a.name, "error", err)
		} else {
			slog.Debug("Rule sink AMQP published message", "name", a.name, "thingId", msg.ThingId)
		}
	}
}

func (a *amqpImpl) setup() error {
	if a.conn.Conn() == nil {
		return fmt.Errorf("sink AMQP connection not established")
	}
	if a.conn.Conn().IsClosed() {
		if err := a.conn.Connect(); err != nil {
			return err
		}
	}
	ch, err := a.conn.Conn().Channel()
	if err != nil {
		return err
	}
	a.channel = ch
	slog.Info("Rule sink AMQP  channel inited")
	return nil
}
