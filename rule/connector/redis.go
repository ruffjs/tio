package connector

import (
	"context"
	"log/slog"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/mitchellh/mapstructure"
)

// Redis connector

const TypeRedis = "redis"

func init() {
	Register(TypeRedis, func(name string, cfg map[string]any) Conn {
		var ac RedisConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			slog.Error("Rule connector redis failed to decode config", "error", err)
			os.Exit(1)
		}
		if ac.Url == "" {
			slog.Error("Rule connector redis config uri is empty")
			os.Exit(1)
		}
		opt, err := redis.ParseURL(ac.Url)
		if err != nil {
			slog.Error("Rule connector redis failed to parse config uri", "uri", ac.Url)
			os.Exit(1)
		}

		opt.MaxRetries = ac.MaxRetries
		c := &Redis{
			name:   name,
			config: ac,
			client: redis.NewClient(opt),
		}
		c.Connect()
		slog.Info("Rule connector Redis inited")
		return c
	})
}

type RedisConfig struct {
	Url        string `json:"url"`
	MaxRetries int    `json:"maxRetries"`
}

type Redis struct {
	name   string
	config RedisConfig
	client *redis.Client
}

func (c *Redis) Status() Status {
	r := c.client.Ping(context.Background())
	if r.Err() != nil {
		return StatusDisconnected
	}
	return StatusConnected
}

func (c *Redis) Name() string {
	return c.name
}

func (*Redis) Type() string {
	return TypeRedis
}

func (c *Redis) Connect() error {
	return nil
}

func (c *Redis) Conn() *redis.Client {
	return c.client
}
