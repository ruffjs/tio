package connector

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"ruff.io/tio/rule/model"
)

// Redis connector

const TypeRedis = "redis"

func init() {
	Register(TypeRedis, newRedis)
}

type RedisConfig struct {
	// redis://user:password@localhost:6789/3?dial_timeout=3&db=1&read_timeout=6s&max_retries=2
	Url string `json:"url"`
}

type Redis struct {
	ctx    context.Context
	name   string
	config RedisConfig
	client *redis.Client

	started bool
	mu      sync.RWMutex
}

func newRedis(ctx context.Context, name string, cfg map[string]any) (Conn, error) {
	var ac RedisConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "failed to decode config")
	}
	if ac.Url == "" {
		return nil, fmt.Errorf("config uri is empty")
	}
	opt, err := redis.ParseURL(ac.Url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config uri: %w", err)
	}

	c := &Redis{
		ctx:    ctx,
		name:   name,
		config: ac,
		client: redis.NewClient(opt),
	}
	slog.Info("Rule connector Redis inited")
	return c, nil
}

func (c *Redis) Status() model.StatusInfo {
	if !c.started {
		return model.StatusNotStarted()
	}
	r := c.client.Ping(context.Background())
	if r.Err() != nil {
		return model.StatusDisconnected(r.Err().Error(), r.Err())
	}
	return model.StatusConnected()
}

func (c *Redis) Name() string {
	return c.name
}

func (*Redis) Type() string {
	return TypeRedis
}

func (c *Redis) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = true
	return c.Status().Error
}

func (c *Redis) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = true
	// TODO Figure out if should close the connecton and if it can be reponded
	// return c.client.Close()
	return nil
}

func (c *Redis) Conn() *redis.Client {
	return c.client
}
