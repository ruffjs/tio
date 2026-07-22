package sink

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
	amqp "github.com/rabbitmq/amqp091-go"
	"ruff.io/tio/connector"
	ruleconnector "ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
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

func NewAmqp(ctx context.Context, name string, cfg map[string]any, ruleConn ruleconnector.Conn, _ connector.Connector) (Sink, error) {
	var ac AmqpConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, fmt.Errorf("decode config %v", err)
	}
	c, ok := ruleConn.(*ruleconnector.Amqp)
	if !ok {
		return nil, fmt.Errorf("wrong connector type for AMQP sink")
	}

	a := &amqpImpl{
		ctx:    ctx,
		name:   name,
		config: ac,
		conn:   c,
		// TODO: Through chan for now, optimized later
		ch:     make(chan *Msg, 100000),
		status: model.StatusNotStarted(),
	}
	go a.publishLoop()
	return a, nil
}

type amqpImpl struct {
	ctx     context.Context
	name    string
	config  AmqpConfig
	ch      chan *Msg
	conn    *ruleconnector.Amqp
	channel *amqp.Channel

	started bool
	status  model.StatusInfo
	mu      sync.Mutex
}

func (a *amqpImpl) Start() error {
	slog.Info("Rule start sink", "type", a.Type(), "name", a.name)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started {
		slog.Info("Rule skip start sink (already started)", "type", a.Type(), "name", a.name)
		return nil
	}
	a.started = true
	slog.Info("Rule sink started", "type", a.Type(), "name", a.name)

	return a.initChannel()
}

func (a *amqpImpl) Stop() error {
	slog.Info("Rule stopping sink", "type", a.Type(), "name", a.name)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.started = false
	a.status = model.StatusNotStarted()
	if len(a.ch) == 0 {
		a.conn.RemoveChannel(a.channelName())
	} else {
		go func() {
			time.Sleep(time.Second)
			a.conn.RemoveChannel(a.channelName())
		}()
	}
	slog.Info("Rule sink stopped", "type", a.Type(), "name", a.name)
	return nil
}

func (a *amqpImpl) Status() model.StatusInfo {
	if !a.started {
		return a.status
	}
	return withConnStatus(a.conn.Name(), a.conn.Status())
}

func (a *amqpImpl) Name() string {
	return a.name
}

func (*amqpImpl) Type() string {
	return TypeAMQP
}

func (a *amqpImpl) Publish(msg Msg) {
	if a.started {
		a.ch <- &msg
	}
}

func (a *amqpImpl) initChannel() error {
	ch, err := a.conn.GetChannel(a.channelName(), func(ch *amqp.Channel) {
		slog.Info("Rule sink AMQP channel updated", "name", a.name, "connectorName", a.conn.Name())
		a.status = model.StatusConnected()
		a.channel = ch
	})
	a.channel = ch
	if err != nil {
		slog.Error("Rule sink AMQP get channel", "name", a.name, "error", err)
	} else {

	}
	return err
}

func (a *amqpImpl) publishLoop() {
LOOP:
	for {
		var msg *Msg
		select {
		case <-a.ctx.Done():
			slog.Debug("Rule sink AMQP publish loop exit cause context done", "name", a.name)
			return
		case msg = <-a.ch:
		}

		// wait connect
		for {
			channelOk := a.channel != nil && !a.channel.IsClosed()
			if channelOk {
				break
			} else {
				qRatio := float64(len(a.ch)) / float64(cap(a.ch))
				if qRatio > 0.5 {
					slog.Error("Rule sink AMQP discard message for channel disconnected", "name", a.name, "message", msg)
					// discard the message for now
					goto LOOP
				}
				time.Sleep(time.Second)
			}
		}

		ctx, cancel := context.WithTimeout(a.ctx, 3*time.Second)
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

func (a *amqpImpl) channelName() string {
	return "sink:" + a.name
}
