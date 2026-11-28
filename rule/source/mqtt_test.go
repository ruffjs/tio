package source

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	ruleconnector "ruff.io/tio/rule/connector"
)

func TestMqttSource_WrongConnector(t *testing.T) {
	_, err := New(context.Background(), Config{
		Name:    "s1",
		Type:    "mqtt",
		Options: map[string]any{"topic": "t"},
	}, ruleconnector.Conn(nil), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong connector type")
}
