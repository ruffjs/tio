package connector

import (
	"log/slog"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
)

const TypeInfluxDB = "influxdb"

func init() {
	Register(TypeInfluxDB, func(name string, cfg map[string]any) Conn {
		var ac InfluxDBConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			slog.Error("Failed to decode config", "error", err)
			os.Exit(1)
		}
		c := &InfluxDB{
			name:   name,
			config: ac,
		}
		c.client = c.initClient()
		return c
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
	name   string
	config InfluxDBConfig
	client *resty.Client
}

func (c *InfluxDB) Status() Status {
	panic("unimplemented")
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
