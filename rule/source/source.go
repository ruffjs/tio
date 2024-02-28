package source

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
)

const (
	TypeEmbedMqtt = "embed-mqtt"
)

type Msg struct {
	ThingId string
	Topic   string
	Payload []byte
}

type MsgHander func(msg Msg)

type Source interface {
	OnMsg(h MsgHander)
	Type() string
}

type Config struct {
	Name      string
	Type      string
	Connector string
	Options   map[string]any
}

func New(cfg Config, conn connector.Conn) (Source, error) {
	switch cfg.Type {
	case TypeEmbedMqtt:
		var mcf EmbedMqttConfig
		if err := mapstructure.Decode(cfg.Options, &mcf); err != nil {
			return nil, fmt.Errorf("decode inner-mqtt options:%v", err)
		}
		s := NewEmbedMqtt(mcf)
		return s, nil
	default:
		return nil, fmt.Errorf("unsupported source type %q", cfg.Type)
	}
}
