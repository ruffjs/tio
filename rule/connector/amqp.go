package connector

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"ruff.io/tio/rule/model"
)

const TypeAMQP = "amqp"

func init() {
	Register(TypeAMQP, NewAmqp)
}

type AmqpConfig struct {
	Url string `json:"url"` // eg: amqp://guest:guest@localhost:5672/
}

func NewAmqp(ctx context.Context, name string, cfg map[string]any) (Conn, error) {
	var ac AmqpConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "decode config")
	}
	a := &Amqp{
		ctx:    ctx,
		name:   name,
		config: ac,
		status: model.StatusNotStarted(),
	}
	return a, nil
}

type Amqp struct {
	ctx    context.Context
	name   string
	config AmqpConfig
	conn   *amqp.Connection

	channels       sync.Map // string -> Channel
	channlesNotify sync.Map // string -> func(ch *amqp.Channel)

	status  model.StatusInfo
	started bool
	mu      sync.RWMutex
}

func (a *Amqp) GetChannel(name string, receiveUpdate func(ch *amqp.Channel)) (*amqp.Channel, error) {
	a.channlesNotify.Store(name, receiveUpdate)
	if c, ok := a.channels.Load(name); ok {
		return c.(*amqp.Channel), nil
	}
	if a.conn == nil || !a.conn.IsClosed() {
		return nil, fmt.Errorf("connection is disconnected")
	}
	if c, err := a.conn.Channel(); err != nil {
		return nil, err
	} else {
		a.channels.Store(name, c)
		return c, nil
	}
}

func (a *Amqp) RemoveChannel(name string) {
	if c, ok := a.channels.Load(name); ok {
		c.(*amqp.Channel).Close()
		a.channels.Delete(name)
	}
}

func (a *Amqp) Name() string {
	return a.name
}

func (a *Amqp) Type() string {
	return TypeAMQP
}

func (a *Amqp) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.started = false
	a.status = model.StatusNotStarted()
	return a.conn.Close()
}

func (a *Amqp) Status() model.StatusInfo {
	return a.status
}

func (a *Amqp) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.started {
		return nil
	}
	a.started = true
	go a.reconnect()
	return nil
}

func (a *Amqp) connect() error {
	conn, err := amqp.Dial(a.config.Url)
	if err != nil {
		slog.Error("Amqp connect", "name", a.name, "error", err)
		a.status = model.StatusDisconnected(err.Error(), err)
		return err
	} else {
		slog.Info("Amqp connection established", "name", a.name)
		a.status = model.StatusConnected()
	}
	a.conn = conn

	closeCh := make(chan *amqp.Error)
	conn.NotifyClose(closeCh)

	// auto reconnect
	go func() {
		select {
		case <-a.ctx.Done():
			return
		case err := <-closeCh:
			if err != nil {
				slog.Error("Amqp connection closed", "name", a.name, "error", err)
				a.status = model.StatusDisconnected(err.Error(), err)
				a.reconnect()
			}
		}
	}()
	return nil
}

// reconnect Keep retrying until success
func (a *Amqp) reconnect() {
	if a.conn != nil {
		a.conn.Close()
	}
	tryCount := 0
	maxSleepTime := time.Second * 20
	for {
		if a.ctx.Err() != nil || !a.started {
			return
		}

		tryCount++
		if err := a.connect(); err != nil {
			t := time.Duration(tryCount*2) * time.Second
			if t > maxSleepTime {
				t = maxSleepTime
			}
			time.Sleep(t)
		} else {
			// update channles
			a.channels.Range(func(chName, _ any) bool {
				a.channels.Delete(chName)
				if newCh, err := a.conn.Channel(); err != nil {
					slog.Error("Rule Amqp open channel failed", "name", a.name, "channelName", chName, "error", err)
				} else {
					a.channels.Store(chName, newCh)
					a.channlesNotify.Range(func(name, value any) bool {
						v := value.(func(ch *amqp.Channel))
						if chName == name {
							v(newCh)
							return false
						}
						return true
					})
				}
				return true
			})
			break
		}
	}
}
