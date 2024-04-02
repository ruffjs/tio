package connector

import (
	"encoding/base64"
	"log/slog"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
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
	name   string
	config TdengineConfig
	client *resty.Client
}

func newTdengine(name string, cfg map[string]any) (Conn, error) {
	var ac TdengineConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("Failed to decode config", "error", err)
		os.Exit(1)
	}
	c := &Tdengine{
		name:   name,
		config: ac,
	}
	c.client = c.initClient()
	return c, nil
}

func (c *Tdengine) Close() error {
	c.client.GetClient().CloseIdleConnections()
	// TODO finish send msg in buffer
	return nil
}

func (c *Tdengine) Status() Status {
	err := testConnectByUrl(c.config.Url)
	if err != nil {
		slog.Error("Rule connector http test connect failed", "name", c.name, "url", c.config.Url, "error", err)
		return StatusDisconnected
	} else {
		return StatusConnected
	}
}

func (c *Tdengine) Name() string {
	return c.name
}

func (*Tdengine) Type() string {
	return TypeTdengine
}

func (c *Tdengine) Connect() error {
	return nil
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
