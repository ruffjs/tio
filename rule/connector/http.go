package connector

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

const TypeHttp = "http"

func init() {
	Register(TypeHttp, func(name string, cfg map[string]any) (Conn, error) {
		var ac HttpConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			return nil, errors.WithMessage(err, "decode config")
		}
		c := &Http{
			name:   name,
			config: ac,
		}
		c.client = c.initClient()
		return c, nil
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

func (c *Http) Close() error {
	c.client.GetClient().CloseIdleConnections()
	return nil
}

func (c *Http) Status() Status {
	err := testConnectByUrl(c.config.Url)
	if err != nil {
		slog.Error("Rule connector http test connect failed", "name", c.name, "url", c.config.Url, "error", err)
		return StatusDisconnected
	} else {
		return StatusConnected
	}
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

func testConnectByUrl(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%s", u.Host, u.Port())
	conn, err := net.DialTimeout("tcp", addr, time.Second*2)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}
