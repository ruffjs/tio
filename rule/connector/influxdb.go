package connector

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"ruff.io/tio/rule/model"
)

const TypeInfluxDB = "influxdb"

func init() {
	Register(TypeInfluxDB, func(ctx context.Context, name string, cfg map[string]any) (Conn, error) {
		var ac InfluxDBConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			return nil, errors.WithMessage(err, "decode config")
		}
		c := &InfluxDB{
			ctx:    ctx,
			name:   name,
			config: ac,
		}
		c.client = c.initClient()
		return c, nil
	})
}

type InfluxDBConfig struct {
	Url           string `json:"url"`
	Token         string `json:"token"`
	Bucket        string `json:"bucket"`
	Org           string `json:"org"`
	TimePrecision string `json:"timePrecision"` // ns us ms s
	Timeout       int    `json:"timeout"`       // in seconds
}

type InfluxDB struct {
	ctx    context.Context
	name   string
	config InfluxDBConfig
	client *resty.Client
}

func (c *InfluxDB) Start() error {
	return c.Status().Error
}

func (c *InfluxDB) Stop() error {
	c.client.GetClient().CloseIdleConnections()
	// TODO finish send msg in buffer
	return nil
}

func (c *InfluxDB) Status() model.StatusInfo {
	err := testConnectByUrl(c.config.Url)
	if err != nil {
		slog.Error("Rule connector http test connect failed", "name", c.name, "url", c.config.Url, "error", err)
		return model.StatusDisconnected(err.Error(), err)
	} else {
		return model.StatusConnected()
	}
}

func (c *InfluxDB) Name() string {
	return c.name
}

func (*InfluxDB) Type() string {
	return TypeInfluxDB
}

func (c *InfluxDB) Connect() error {
	return nil
}

func (c *InfluxDB) Client() *resty.Client {
	return c.client
}

func (c *InfluxDB) initClient() *resty.Client {
	cl := resty.New().
		SetBaseURL(c.config.Url+"/api/v2/write").
		SetQueryParam("org", c.config.Org).
		SetQueryParam("bucket", c.config.Bucket).
		SetQueryParam("precision", c.config.TimePrecision).
		SetHeader("Authorization", "Token "+c.config.Token).
		SetHeader("Content-Type", "text/plain; charset=utf-8").
		SetHeader("Accept", "application/json").
		SetTimeout(time.Duration(c.config.Timeout) * time.Second)

	return cl
}
