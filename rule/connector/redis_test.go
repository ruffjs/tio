package connector

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedisCommands(t *testing.T) {
	server := miniredis.RunT(t)
	server.RequireAuth("secret")
	ctx := context.Background()
	conn, err := newRedis(ctx, "test", map[string]any{
		"url": "redis://:secret@" + server.Addr() + "/2",
	})
	require.NoError(t, err)
	client := conn.(*Redis).Conn()
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	require.NoError(t, conn.Start())
	require.NoError(t, client.Do(ctx, "SET", "key", "value").Err())
	value, err := client.Do(ctx, "GET", "key").Text()
	require.NoError(t, err)
	require.Equal(t, "value", value)
	server.Select(2)
	value, err = server.Get("key")
	require.NoError(t, err)
	require.Equal(t, "value", value)
}
