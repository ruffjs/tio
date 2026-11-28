package config

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigDefaults(t *testing.T) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(t.TempDir())
	cfg, err := loadConfig(v)
	require.NoError(t, err)
	require.False(t, cfg.Nats.Enabled)
	require.Equal(t, "embedded", cfg.Nats.Mode)
	require.Equal(t, 4222, cfg.Nats.Server.Port)
	require.Equal(t, 1883, cfg.Connector.MqttBroker.TcpPort)
	require.Equal(t, 8083, cfg.Connector.MqttBroker.WsPort)
	require.Equal(t, []UserPassword{{Name: "$biz", Password: "public"}, {Name: "$tio", Password: "public"}}, cfg.Connector.MqttBroker.SuperUsers)
	require.Equal(t, "./data/nats", cfg.Nats.JetStream.StoreDir)
	require.Equal(t, 80000, cfg.Nats.JetStream.DefaultMaxConsumers)
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	nats := filepath.Join(dir, "nats.yaml")
	require.NoError(t, os.WriteFile(main, []byte("api:\n  port: 9100\nnats:\n  enabled: true\n  configFile: ./nats.yaml\n"), 0600))
	require.NoError(t, os.WriteFile(nats, []byte("nats:\n  server:\n    port: 14222\n  jetStream:\n    enabled: true\n    defaultMaxConsumers: 25000\n"), 0600))
	cfg, err := LoadConfigFile(main)
	require.NoError(t, err)
	require.Equal(t, 9100, cfg.API.Port)
	require.True(t, cfg.Nats.Enabled)
	require.Equal(t, nats, cfg.Nats.ConfigFile)
	require.Equal(t, 14222, cfg.Nats.Server.Port)
	require.True(t, cfg.Nats.JetStream.Enabled)
	require.Equal(t, 25000, cfg.Nats.JetStream.DefaultMaxConsumers)
	require.Equal(t, "$tio-app", cfg.Nats.AppClient.User)

	// Absolute configFile reference is accepted.
	require.NoError(t, os.WriteFile(main, []byte("nats:\n  configFile: "+nats+"\n"), 0600))
	require.NoError(t, os.WriteFile(nats, []byte("nats:\n  server:\n    port: 14223\n"), 0600))
	cfg, err = LoadConfigFile(main)
	require.NoError(t, err)
	require.Equal(t, 14223, cfg.Nats.Server.Port)
}

func TestLoadConfigNatsConfigFileOverridesJetStream(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	nats := filepath.Join(dir, "nats.yaml")
	require.NoError(t, os.WriteFile(main, []byte("nats:\n  enabled: true\n  configFile: ./nats.yaml\n"), 0600))
	require.NoError(t, os.WriteFile(nats, []byte("nats:\n  jetStream:\n    enabled: false\n    storeDir: /tmp/tio-nats\n"), 0600))
	cfg, err := LoadConfigFile(main)
	require.NoError(t, err)
	require.False(t, cfg.Nats.JetStream.Enabled)
	require.Equal(t, "/tmp/tio-nats", cfg.Nats.JetStream.StoreDir)
}

func TestLoadConfigErrors(t *testing.T) {
	cases := []struct {
		name, main, nats, want string
	}{
		{"old encoding key", "protocol:\n  encoding: cbor\n", "", "protocol.defaultEncoding"},
		{"invalid main", "api: [\n", "", "read main config"},
		{"missing NATS file", "nats:\n  configFile: missing.yaml\n", "", "read NATS config"},
		{"NATS path is a directory", "nats:\n  configFile: .\n", "", "read NATS config"},
		{"invalid NATS YAML", "nats:\n  configFile: nats.yaml\n", "nats: [\n", "read NATS config"},
		{"unknown NATS option", "nats:\n  configFile: nats.yaml\n", "nats:\n  server:\n    unknownOpt: 1\n", "decode NATS config"},
		{"invalid consumer limit", "nats:\n  enabled: true\n  configFile: nats.yaml\n", "nats:\n  jetStream:\n    defaultMaxConsumers: -2\n", "defaultMaxConsumers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			filename := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(filename, []byte(tc.main), 0600))
			if tc.nats != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "nats.yaml"), []byte(tc.nats), 0600))
			}
			_, err := LoadConfigFile(filename)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestValidateNatsConfig_DisabledSkipsValidation(t *testing.T) {
	require.NoError(t, ValidateNatsConfig(NatsConfig{Enabled: false}, DBMySQL))
	require.NoError(t, ValidateNatsConfig(NatsConfig{Enabled: false, Server: NatsServerConfig{Routes: []string{"nats://a:6222"}}}, DBSqlite))
}

func TestValidateNatsConfig_MultiNodeWithSQLite(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.Routes = []string{"nats://node-2:6222"}
	err := ValidateNatsConfig(cfg, DBSqlite)
	require.Error(t, err)
	require.ErrorContains(t, err, "sqlite")
}

func TestShippedConfigs(t *testing.T) {
	for _, filename := range []string{
		"../config.yaml",
		"../integration_tests/config-test.yaml",
		"../demos/mtls/config.mtls.yaml",
		"../deployments/nats-cluster/configs/node-1.yaml",
		"../deployments/nats-cluster/configs/node-2.yaml",
		"../deployments/nats-cluster/configs/node-3.yaml",
	} {
		t.Run(filename, func(t *testing.T) {
			_, err := LoadConfigFile(filename)
			require.NoError(t, err)
		})
	}
}

func TestLoadNatsDefaultsFile(t *testing.T) {
	defaultsFile, err := filepath.Abs("../config-nats.default.yaml")
	require.NoError(t, err)
	filename := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(filename, []byte("nats:\n  configFile: "+defaultsFile+"\n"), 0600))
	cfg, err := LoadConfigFile(filename)
	require.NoError(t, err)
	defaults, err := LoadConfigFile("../config.yaml")
	require.NoError(t, err)
	defaults.Nats.ConfigFile = defaultsFile
	require.Equal(t, defaults.Nats, cfg.Nats)
}

func TestConfigMarshalOmitsSecrets(t *testing.T) {
	cfg := Config{
		ProvisionSecret: "super-secret",
	}
	cfg.API.BasicAuth.Name = "admin"
	cfg.API.BasicAuth.Password = "pass"
	cfg.DB.Mysql.Password = "mysql-pass"
	cfg.DB.Sqlite.FilePath = "tio.sqlite"
	cfg.Connector.MqttBroker.SuperUsers = []UserPassword{{Name: "root", Password: "root-pass"}}
	cfg.Nats.AppClient = NatsClientConfig{User: "app", Password: "app-secret"}
	cfg.Nats.SystemClient = NatsClientConfig{User: "sys", Password: "sys-secret"}
	cfg.Nats.Server.ClusterPassword = "route-secret"

	b, err := json.Marshal(cfg, json.Deterministic(true), json.FormatNilMapAsNull(true), json.FormatNilSliceAsNull(true))
	require.NoError(t, err)
	got := string(b)
	for _, secret := range []string{"super-secret", "pass", "mysql-pass", "root-pass", "app-secret", "sys-secret", "route-secret"} {
		require.NotContains(t, got, secret)
	}
	require.NotContains(t, got, "ProvisionSecret")
}

func validSingleNatsConfig() NatsConfig {
	return NatsConfig{
		Enabled: true,
		Mode:    "embedded",
		Server: NatsServerConfig{
			ServerName:      "tio-node-1",
			ClusterName:     "tio",
			Port:            4222,
			MonitorPort:     0,
			ClusterUser:     "$tio-route",
			ClusterPassword: "public",
		},
		AppClient:    NatsClientConfig{User: "$tio-app", Password: "public"},
		SystemClient: NatsClientConfig{User: "$tio-sys", Password: "public"},
		JetStream:    NatsJetStreamConfig{Enabled: true, StoreDir: "./data/nats"},
	}
}

func TestValidateNatsConfig_ValidSingleNode(t *testing.T) {
	require.NoError(t, ValidateNatsConfig(validSingleNatsConfig(), DBSqlite))
}

func TestValidateNatsConfig_ValidMultiNodeWithMySQL(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.Routes = []string{"nats://node-2:6222", "nats://node-3:6222"}
	require.NoError(t, ValidateNatsConfig(cfg, DBMySQL))
}

func TestValidateNatsConfig_EmptyServerName(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.ServerName = ""
	err := ValidateNatsConfig(cfg, DBSqlite)
	require.Error(t, err)
	require.ErrorContains(t, err, "serverName")
}

func TestValidateNatsConfig_EmptyClusterName(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.ClusterName = ""
	require.Error(t, ValidateNatsConfig(cfg, DBSqlite))
}

func TestValidateNatsConfig_IncompleteRouteTLS(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.RouteTLS = NatsTLSConfig{CertFile: "/path/cert.pem"}
	err := ValidateNatsConfig(cfg, DBSqlite)
	require.Error(t, err)
	require.ErrorContains(t, err, "routeTls")
}

func TestValidateNatsConfig_EmptyClientUser(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NatsConfig)
		field  string
	}{
		{"empty appClient user", func(c *NatsConfig) { c.AppClient.User = "" }, "appClient"},
		{"empty systemClient user", func(c *NatsConfig) { c.SystemClient.User = "" }, "systemClient"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validSingleNatsConfig()
			tt.mutate(&cfg)
			err := ValidateNatsConfig(cfg, DBSqlite)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.field)
		})
	}
}

func TestValidateNatsConfig_EmptyJetStreamStoreDir(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.JetStream.StoreDir = ""
	err := ValidateNatsConfig(cfg, DBSqlite)
	require.Error(t, err)
	require.ErrorContains(t, err, "storeDir")
}

func TestProtocolValidate(t *testing.T) {
	tests := []struct {
		name    string
		p       Protocol
		wantErr bool
	}{
		{"json_legacy", Protocol{DefaultEncoding: "json", Mode: "legacy"}, false},
		{"json_simple", Protocol{DefaultEncoding: "json", Mode: "simple"}, false},
		{"cbor_legacy", Protocol{DefaultEncoding: "cbor", Mode: "legacy"}, false},
		{"cbor_simple", Protocol{DefaultEncoding: "cbor", Mode: "simple"}, false},
		{"json_dual", Protocol{DefaultEncoding: "json", Mode: "dual"}, false},
		{"cbor_dual", Protocol{DefaultEncoding: "cbor", Mode: "dual"}, false},
		{"unknown_encoding", Protocol{DefaultEncoding: "xml", Mode: "legacy"}, true},
		{"empty_encoding", Protocol{DefaultEncoding: "", Mode: "legacy"}, true},
		{"unknown_mode", Protocol{DefaultEncoding: "json", Mode: "fast"}, true},
		{"empty_mode", Protocol{DefaultEncoding: "json", Mode: ""}, true},
		{"both_empty", Protocol{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateNatsConfigRejectsEmptyPasswords(t *testing.T) {
	cfg, err := LoadConfigFile("../config.yaml")
	require.NoError(t, err)
	n := cfg.Nats
	n.Enabled = true
	n.JetStream.Enabled = true
	n.JetStream.StoreDir = "./data/nats"
	require.NoError(t, ValidateNatsConfig(n, DBMySQL))

	n2 := n
	n2.AppClient.Password = ""
	require.Error(t, ValidateNatsConfig(n2, DBMySQL))

	n3 := n
	n3.SystemClient.Password = ""
	require.Error(t, ValidateNatsConfig(n3, DBMySQL))
}
