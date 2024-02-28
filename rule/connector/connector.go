package connector

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
)

const (
	TypeAMQP = "amqp"
	// more type in future
)

type Conn interface {
	Type() string
	Setup() error
}

type Config struct {
	Name    string
	Type    string
	Options map[string]any
}

func New(cfg Config) (Conn, error) {
	switch cfg.Type {
	case TypeAMQP:
		var ac AmqpConfig
		if err := mapstructure.Decode(cfg.Options, &ac); err != nil {
			return nil, err
		}
		c := NewAmqp(cfg.Name, ac)
		return c, nil
	default:
		return nil, fmt.Errorf("unsupported connector type: %v", cfg.Type)
	}
}
