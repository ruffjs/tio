package connector

import (
	"context"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"ruff.io/tio/rule/model"
)

const TypeTdengine = "tdengine"

func init() {
	Register(TypeTdengine, newTdengine)
}

type TdengineConfig struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Timezone string `json:"timezone"`
	Database string `json:"database"`
	Timeout  int    `json:"timeout"` // seconds
}

type Tdengine struct {
	ctx     context.Context
	name    string
	config  TdengineConfig
	client  *resty.Client
	started bool
}

func newTdengine(ctx context.Context, name string, cfg map[string]any) (Conn, error) {
	var ac TdengineConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "failed to decode config")
	}
	c := &Tdengine{
		ctx:    ctx,
		name:   name,
		config: ac,
	}
	c.client = c.initClient()
	return c, nil
}

func (c *Tdengine) Start() error {
	c.started = true
	return c.Status().Error
}

func (c *Tdengine) Stop() error {
	c.started = false
	c.client.GetClient().CloseIdleConnections()
	// TODO finish send msg in buffer
	return nil
}

func (c *Tdengine) Status() model.StatusInfo {
	if !c.started {
		return model.StatusNotStarted()
	}

	err := testConnectByUrl(c.config.Url)
	if err != nil {
		slog.Error("Rule connector http test connect failed", "name", c.name, "url", c.config.Url, "error", err)
		return model.StatusDisconnected(err.Error(), err)
	} else {
		return model.StatusConnected()
	}
}

func (c *Tdengine) Name() string {
	return c.name
}

func (*Tdengine) Type() string {
	return TypeTdengine
}

func (c *Tdengine) Client() *resty.Client {
	return c.client
}

func (c *Tdengine) initClient() *resty.Client {
	url := c.config.Url + "/rest/sql/" + c.config.Database
	tk := base64.StdEncoding.EncodeToString([]byte(c.config.Username + ":" + c.config.Password))
	auth := "Basic " + tk
	cl := resty.New().
		SetBaseURL(url).
		SetQueryParam("tz", c.config.Timezone).
		SetHeader("Authorization", auth).
		SetHeader("Content-Type", "text/plain; charset=utf-8").
		SetHeader("Accept", "application/json").
		SetTimeout(time.Duration(c.config.Timeout) * time.Second)
	return cl
}
