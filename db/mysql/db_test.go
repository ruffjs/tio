package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDSNReadTimeout(t *testing.T) {
	cfg := Config{
		Host: "127.0.0.1", Port: "3306", User: "u", Password: "p", DB: "d",
		Charset: "utf8mb4", Timezone: "UTC",
	}

	business := dsn(cfg, true)
	require.Contains(t, business, "timeout=5s")
	require.Contains(t, business, "readTimeout=10s")
	require.Contains(t, business, "writeTimeout=10s")

	// Migrations run DDL that can take longer than a normal read.
	migration := dsn(cfg, false)
	require.Contains(t, migration, "timeout=5s")
	require.NotContains(t, migration, "readTimeout")
}
