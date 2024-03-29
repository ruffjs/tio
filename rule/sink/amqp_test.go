package sink_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/sink"
)

func TestAmqp(t *testing.T) {
	if os.Getenv("TEST_AMQP") == "" {
		return
	}

	slog.SetLogLoggerLevel(slog.LevelDebug)
	cfg := &sink.AmqpConfig{
		Exchange:   "test",
		RoutingKey: "route",
	}
	connCfg := &connector.AmqpConfig{
		Url: "amqp://guest:guest@localhost:5672/",
	}
	conn := connector.NewAmqp("test", *connCfg)
	con, ok := conn.(*connector.Amqp)
	require.True(t, ok)
	c := sink.NewAmqp("test", *cfg, con)
	c.Publish(sink.Msg{
		ThingId: "thing",
		Payload: []byte(`{"a": 1}`),
	})
	time.Sleep(time.Millisecond * 100)
}
