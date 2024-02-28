package sink

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
)

const (
	TypeAMQP = "amqp"
	// more type in future
)

type Msg struct {
	ThingId string
	Topic   string
	Payload []byte
}
type Sink interface {
	Publish(msg Msg)
	Type() string
}

type Config struct {
	Name      string
	Type      string
	Connector string
	Options   map[string]any
}

func New(cfg Config, conn connector.Conn) (Sink, error) {
	switch cfg.Type {
	case "amqp":
		var ac AmqpConfig
		if err := mapstructure.Decode(cfg.Options, &ac); err != nil {
			return nil, fmt.Errorf("decode sink amqp config, name=%q, err=%v", cfg.Name, err)
		}
		c, ok := conn.(*connector.Amqp)
		if !ok {
			return nil, fmt.Errorf("wrong connector for amqp sink")
		}
		s := NewAmqp(cfg.Name, ac, c)
		return s, nil
	default:
		return nil, fmt.Errorf("unsupported sink type %q", cfg.Type)
	}
}
