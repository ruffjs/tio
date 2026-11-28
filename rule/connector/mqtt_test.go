package connector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMqttConnector_DecodeConfig(t *testing.T) {
	conn, err := New(context.Background(), Config{
		Name: "c1",
		Type: "mqtt",
		Options: map[string]any{
			"clientId": "id1",
			"host":     "127.0.0.1",
			"port":     1883,
			"user":     "u",
			"password": "p",
		},
	})
	require.NoError(t, err)
	m, ok := conn.(*Mqtt)
	require.True(t, ok)
	require.Equal(t, "id1", m.config.ClientId)
	require.Equal(t, "127.0.0.1", m.config.Host)
	require.Equal(t, 1883, m.config.Port)
}

func TestMqttConnector_UnknownType(t *testing.T) {
	_, err := New(context.Background(), Config{Name: "x", Type: "nope"})
	require.Error(t, err)
}
