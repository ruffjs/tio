package connector

import (
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AmqpConfig struct {
	Url string `json:"url"` // eg: amqp://guest:guest@localhost:5672/
}

type Amqp struct {
	name   string
	config AmqpConfig
	conn   *amqp.Connection
}

func NewAmqp(name string, c AmqpConfig) Conn {
	a := &Amqp{
		config: c,
	}
	a.Setup()
	return a
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

func (a *Amqp) Setup() error {
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
