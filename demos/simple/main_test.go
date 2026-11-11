package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/config"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/shadow"
)

func TestDevicePassword(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "device database.sqlite")
	t.Setenv("TIO_SQLITE", filename)
	t.Setenv("TIO_DEVICE_PASSWORD", "")
	db, err := sql.Open("sqlite3", filename)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE thing (id TEXT PRIMARY KEY, auth_value TEXT, auth_type TEXT, enabled BOOLEAN)")
	require.NoError(t, err)
	for _, row := range []struct {
		id, authType string
		enabled      bool
	}{
		{"test", "password", true}, {"quoted'id", "password", true},
		{"disabled", "password", false}, {"certificate", "certificate", true},
	} {
		_, err = db.Exec("INSERT INTO thing VALUES (?, ?, ?, ?)", row.id, "fixture-secret", row.authType, row.enabled)
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())
	for _, id := range []string{"test", "quoted'id"} {
		password, err := devicePassword(id)
		require.NoError(t, err)
		require.Equal(t, "fixture-secret", password)
	}
	for _, id := range []string{"missing", "disabled", "certificate"} {
		_, err := devicePassword(id)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "fixture-secret")
	}
	t.Setenv("TIO_SQLITE", filepath.Join(t.TempDir(), "missing.sqlite"))
	_, err = devicePassword("test")
	require.Error(t, err)
	_, err = os.Stat(os.Getenv("TIO_SQLITE"))
	require.True(t, os.IsNotExist(err), "read-only access must not create a database")
	t.Setenv("TIO_DEVICE_PASSWORD", "override-secret")
	password, err := devicePassword("test")
	require.NoError(t, err)
	require.Equal(t, "override-secret", password)
}

func TestNestedDesiredProperty(t *testing.T) {
	baseline := nestedState("reported", "marker")
	patch := map[string]any{"control": map[string]any{"target": map[string]any{"value": "desired"}}}
	merged, changed := shadow.MergePatch(baseline, patch)
	require.True(t, changed)
	require.Equal(t, nestedState("desired", "marker"), merged)
}

func TestAPIConfigResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/private/api/config", r.URL.Path)
		var cfg config.Config
		cfg.Protocol.Mode = "simple"
		cfg.Protocol.DefaultEncoding = "json"
		rest.SendRespOK(w, cfg)
	}))
	defer server.Close()
	p := &probe{apiURL: server.URL, http: server.Client()}
	var cfg config.Config
	require.NoError(t, p.api(http.MethodGet, "/private/api/config", nil, &cfg))
	require.Equal(t, "simple", cfg.Protocol.Mode)
	require.Equal(t, "json", cfg.Protocol.DefaultEncoding)
}
