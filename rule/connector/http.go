package connector

import (
	"log/slog"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
)

const TypeHttp = "http"

func init() {
	Register(TypeHttp, func(name string, cfg map[string]any) Conn {
		var ac HttpConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			slog.Error("Failed to decode config", "error", err)
			os.Exit(1)
		}
		c := &Http{
			name:   name,
			config: ac,
		}
		c.client = c.initClient()
		return c
	})
}

type HttpConfig struct {
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Timeout int               `json:"timeout"` // in seconds
}

type Http struct {
	name   string
	config HttpConfig
	client *resty.Client
}

func (c *Http) Status() Status {
	panic("unimplemented")
}

func (c *Http) Name() string {
	return c.name
}

func (*Http) Type() string {
	return TypeHttp
}

func (c *Http) Connect() error {
	return nil
}

func (c *Http) Client() *resty.Client {
	return c.client
}

func (c *Http) initClient() *resty.Client {
	return resty.New().
		SetBaseURL(c.config.Url).
		SetHeaders(c.config.Headers).
		SetTimeout(time.Duration(c.config.Timeout) * time.Second)
}
