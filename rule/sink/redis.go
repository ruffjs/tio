package sink

import (
	"context"
	"log/slog"
	"os"

	"ruff.io/tio/pkg/redissplit"
	"ruff.io/tio/rule/connector"
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

func NewRedis(name string, cfg map[string]any, conn connector.Conn) Sink {
	c, ok := conn.(*connector.Redis)
	if !ok {
		slog.Error("Rule sink Redis wrong connector type", "sinkName", name)
		os.Exit(1)
	}

	a := &redisImpl{
		name: name,
		conn: c,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a
}

type redisImpl struct {
	name string
	conn *connector.Redis
	ch   chan *Msg
}

func (s *redisImpl) Name() string {
	return s.name
}

func (*redisImpl) Type() string {
	return TypeRedis
}

func (s *redisImpl) Publish(msg Msg) {
	s.ch <- &msg
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

		re := s.conn.Conn().Do(context.Background(), args...)
		if re.Err() != nil {
			slog.Error("Redis sink process failed", "error", re.Err())
		} else {
			slog.Debug("Redis sink process succeeded", "payload", cmd)
		}
	}
}
