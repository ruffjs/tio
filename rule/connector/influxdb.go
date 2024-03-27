package connector

import (
	"log/slog"
	"os"
	"time"

	"github.com/imroc/req/v3"
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
	client *req.Client
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

func (c *InfluxDB) Client() *req.Client {
	return c.client
}

func (c *InfluxDB) initClient() *req.Client {
	cl := req.C().
		SetBaseURL(c.config.Url+"/api/v2/write").
		SetCommonQueryParam("org", c.config.Org).
		SetCommonQueryParam("bucket", c.config.Bucket).
		SetCommonQueryParam("precision", c.config.TimePrecision).
		SetCommonHeader("Authorization", "Token "+c.config.Token).
		SetCommonHeader("Content-Type", "text/plain; charset=utf-8").
		SetCommonHeader("Accept", "application/json")
	cl.SetTimeout(time.Duration(c.config.Timeout) * time.Second)

	return cl
}
