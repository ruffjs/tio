package connector

import (
	"log/slog"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

const TypeAMQP = "amqp"

func init() {
	Register(TypeAMQP, NewAmqp)
}

type AmqpConfig struct {
	Url string `json:"url"` // eg: amqp://guest:guest@localhost:5672/
}

func NewAmqp(name string, cfg map[string]any) (Conn, error) {
	var ac AmqpConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "decode config")
	}
	a := &Amqp{
		config: ac,
	}
	a.Connect()
	return a, nil
}

type Amqp struct {
	name   string
	config AmqpConfig
	conn   *amqp.Connection
}

func (a *Amqp) Close() error {
	return a.conn.Close()
}

func (a *Amqp) Status() Status {
	s := StatusDisconnected
	if !a.conn.IsClosed() {
		s = StatusConnected
	}
	return s
}

func (a *Amqp) Conn() *amqp.Connection {
	return a.conn
}

func (a *Amqp) Name() string {
	return a.name
}

func (a *Amqp) Type() string {
	return TypeAMQP
}

func (a *Amqp) Connect() error {
	conn, err := amqp.Dial(a.config.Url)
	if err != nil {
		slog.Error("Amqp connect", "name", a.name, "error", err)
		return err
	} else {
		slog.Info("Amqp connection established", "name", a.name)
	}
	a.conn = conn
	return nil
}
