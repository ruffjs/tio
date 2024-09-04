package sink

import (
	"context"
	"fmt"
	"log/slog"

	"ruff.io/tio/pkg/redissplit"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

// Redis sink, use raw redis command, like "SET k hi"

// Example
//   - input: {
// 							"payload": {
// 								"sn": "wm-liu",
// 								"data": {
// 									"temp": 112,
// 									"hum": 50
// 								}
// 							}
// 						}
//
//   - jq:   .payload as {sn: $sn, data: $data} | $data
//            | to_entries
//            | map(.key + " " + (.value|tostring))
//            | join(" ")
//            | "HSET prp:" + $sn + " " + .
//
//   - output: HSET prp:wm-liu hum 50 temp 112

const TypeRedis = "redis"

func init() {
	Register(TypeRedis, NewRedis)
}

func NewRedis(ctx context.Context, name string, cfg map[string]any, conn connector.Conn) (Sink, error) {
	c, ok := conn.(*connector.Redis)
	if !ok {
		return nil, fmt.Errorf("wrong connector type for Redis sink")
	}

	a := &redisImpl{
		ctx:  ctx,
		name: name,
		conn: c,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a, nil
}

type redisImpl struct {
	ctx  context.Context
	name string
	conn *connector.Redis
	ch   chan *Msg

	started bool
}

func (s *redisImpl) Start() error {
	s.started = true
	return s.Status().Error
}

func (s *redisImpl) Status() model.StatusInfo {
	if !s.started {
		return model.StatusNotStarted()
	}
	return withConnStatus(s.conn.Name(), s.conn.Status())
}

func (s *redisImpl) Stop() error {
	s.started = false
	return nil
}

func (s *redisImpl) Name() string {
	return s.name
}

func (*redisImpl) Type() string {
	return TypeRedis
}

func (s *redisImpl) Publish(msg Msg) {
	if s.started {
		s.ch <- &msg
	}
}

func (s *redisImpl) publishLoop() {
	for {
		msg := <-s.ch
		cmd := string(msg.Payload)
		sa, err := redissplit.SplitArgs(cmd)
		if err != nil {
			slog.Error("Redis sink split cmd string", "error", err)
			return
		}
		args := make([]any, len(sa))
		for i, v := range sa {
			args[i] = v
		}

		re := s.conn.Conn().Do(s.ctx, args...)
		if re.Err() != nil {
			slog.Error("Redis sink process failed", "error", re.Err())
		} else {
			slog.Debug("Redis sink process succeeded", "payload", cmd)
		}
	}
}
