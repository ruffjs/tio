package connector

import (
	"context"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"ruff.io/tio/rule/model"
)

const TypeHttp = "http"

func init() {
	Register(TypeHttp, func(ctx context.Context, name string, cfg map[string]any) (Conn, error) {
		var ac HttpConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			return nil, errors.WithMessage(err, "decode config")
		}
		c := &Http{
			ctx:    ctx,
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
	ctx    context.Context
	name   string
	config HttpConfig
	client *resty.Client

	started           bool
	status            model.StatusInfo
	statusLastRefresh time.Time

	mu sync.RWMutex
}

func (c *Http) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = true
	return c.Status().Error
}

func (c *Http) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = false
	c.client.GetClient().CloseIdleConnections()
	return nil
}

func (c *Http) Status() model.StatusInfo {
	if !c.started {
		return model.StatusNotStarted()
	}
	if time.Since(c.statusLastRefresh).Seconds() < 10 {
		return c.status
	}

	err := testConnectByUrl(c.config.Url)
	if err != nil {
		slog.Error("Rule connector http test connect failed", "name", c.name, "url", c.config.Url, "error", err)
		c.status = model.StatusDisconnected(err.Error(), err)
	} else {
		c.status = model.StatusConnected()
	}
	c.statusLastRefresh = time.Now()
	return c.status
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
	addr := u.Host
	if u.Port() == "" {
		addr = addr + ":80"
	}
	conn, err := net.DialTimeout("tcp", addr, time.Second*2)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}
