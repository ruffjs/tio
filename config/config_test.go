package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigMarshalOmitsSecrets(t *testing.T) {
	cfg := Config{
		ProvisionSecret: "super-secret",
	}
	cfg.API.BasicAuth.Name = "admin"
	cfg.API.BasicAuth.Password = "pass"
	cfg.DB.Mysql.Password = "mysql-pass"
	cfg.DB.Sqlite.FilePath = "tio.sqlite"
	cfg.Connector.MqttClient.Password = "mqtt-pass"
	cfg.Connector.Emqx.ApiPassword = "emqx-pass"
	cfg.Connector.MqttBroker.SuperUsers = []UserPassword{{Name: "root", Password: "root-pass"}}

	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	got := string(b)
	if strings.Contains(got, "super-secret") {
		t.Fatalf("provision secret should not be marshaled: %s", got)
	}
	if strings.Contains(got, "pass") {
		t.Fatalf("password fields should not be marshaled: %s", got)
	}
	if strings.Contains(got, "ProvisionSecret") {
		t.Fatalf("provision secret field name should not be marshaled: %s", got)
	}
}

func TestNatsConfigMarshalOmitsSecrets(t *testing.T) {
	cfg := NatsConfig{
		Server: NatsServerConfig{
			ClusterPassword: "route-secret",
		},
		AppClient:     NatsClientConfig{User: "app", Password: "app-secret"},
		SystemClient:  NatsClientConfig{User: "sys", Password: "sys-secret"},
		MqttPublisher: NatsClientConfig{User: "pub", Password: "pub-secret"},
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal nats config: %v", err)
	}

	got := string(b)
	for _, secret := range []string{"route-secret", "app-secret", "sys-secret", "pub-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("nats secret %q should not be marshaled: %s", secret, got)
		}
	}
}

func validSingleNatsConfig() NatsConfig {
	return NatsConfig{
		Server: NatsServerConfig{
			ServerName:         "tio-node-1",
			ClusterName:        "tio",
			Port:               4222,
			ClusterPort:        6222,
			MqttPort:           1883,
			WsPort:             8083,
			StoreDir:           "./data/nats/tio-node-1",
			JetStreamMaxMemory: 268435456,
			JetStreamMaxStore:  1073741824,
			MqttStreamReplicas: 1,
			PresenceReplicas:   1,
			ClusterUser:        "$tio-route",
			ClusterPassword:    "public",
		},
		AppClient:     NatsClientConfig{User: "$tio-app", Password: "public"},
		SystemClient:  NatsClientConfig{User: "$tio-sys", Password: "public"},
		MqttPublisher: NatsClientConfig{User: "$tio-pub", Password: "public"},
	}
}

func TestValidateNatsConfig_ValidSingleNode(t *testing.T) {
	cfg := validSingleNatsConfig()
	if err := ValidateNatsConfig(cfg, DBSqlite); err != nil {
		t.Fatalf("expected valid single-node config, got: %v", err)
	}
}

func TestValidateNatsConfig_ValidMultiNodeWithMySQL(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.Routes = []string{"nats://node-2:6222", "nats://node-3:6222"}
	cfg.Server.MqttStreamReplicas = 3
	cfg.Server.PresenceReplicas = 3

	if err := ValidateNatsConfig(cfg, DBMySQL); err != nil {
		t.Fatalf("expected valid multi-node config with mysql, got: %v", err)
	}
}

func TestValidateNatsConfig_EmptyServerName(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.ServerName = ""

	err := ValidateNatsConfig(cfg, DBSqlite)
	if err == nil {
		t.Fatal("expected error for empty serverName")
	}
	if !strings.Contains(err.Error(), "serverName") {
		t.Fatalf("expected serverName in error, got: %v", err)
	}
}

func TestValidateNatsConfig_EmptyClusterName(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.ClusterName = ""

	err := ValidateNatsConfig(cfg, DBSqlite)
	if err == nil {
		t.Fatal("expected error for empty clusterName")
	}
}

func TestValidateNatsConfig_MultiNodeWithSQLite(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.Routes = []string{"nats://node-2:6222"}
	cfg.Server.MqttStreamReplicas = 3
	cfg.Server.PresenceReplicas = 3

	err := ValidateNatsConfig(cfg, DBSqlite)
	if err == nil {
		t.Fatal("expected error for multi-node with sqlite")
	}
	if !strings.Contains(err.Error(), "sqlite") {
		t.Fatalf("expected sqlite in error, got: %v", err)
	}
}

func TestValidateNatsConfig_InvalidReplicaCountSingleNode(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.MqttStreamReplicas = 3

	err := ValidateNatsConfig(cfg, DBSqlite)
	if err == nil {
		t.Fatal("expected error for invalid mqttStreamReplicas on single node")
	}
}

func TestValidateNatsConfig_InvalidReplicaCountMultiNode(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.Routes = []string{"nats://node-2:6222"}
	cfg.Server.MqttStreamReplicas = 1
	cfg.Server.PresenceReplicas = 3

	err := ValidateNatsConfig(cfg, DBMySQL)
	if err == nil {
		t.Fatal("expected error for invalid mqttStreamReplicas on multi-node")
	}
}

func TestValidateNatsConfig_PlaintextAndTLSMqtt(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.MqttPort = 1883
	cfg.Server.MqttTLS = NatsTLSConfig{
		CertFile: "/path/cert.pem",
		KeyFile:  "/path/key.pem",
		CAFile:   "/path/ca.pem",
	}

	err := ValidateNatsConfig(cfg, DBSqlite)
	if err == nil {
		t.Fatal("expected error for both plaintext and TLS mqtt listeners")
	}
}

func TestValidateNatsConfig_IncompleteRouteTLS(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.RouteTLS = NatsTLSConfig{
		CertFile: "/path/cert.pem",
	}

	err := ValidateNatsConfig(cfg, DBSqlite)
	if err == nil {
		t.Fatal("expected error for incomplete route TLS")
	}
	if !strings.Contains(err.Error(), "routeTls") {
		t.Fatalf("expected routeTls in error, got: %v", err)
	}
}

func TestValidateNatsConfig_EmptyClientUser(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NatsConfig)
		field  string
	}{
		{"empty appClient user", func(c *NatsConfig) { c.AppClient.User = "" }, "appClient"},
		{"empty systemClient user", func(c *NatsConfig) { c.SystemClient.User = "" }, "systemClient"},
		{"empty mqttPublisher user", func(c *NatsConfig) { c.MqttPublisher.User = "" }, "mqttPublisher"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validSingleNatsConfig()
			tt.mutate(&cfg)
			err := ValidateNatsConfig(cfg, DBSqlite)
			if err == nil {
				t.Fatalf("expected error for %s", tt.field)
			}
			if !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected %s in error, got: %v", tt.field, err)
			}
		})
	}
}

func TestValidateNatsConfig_EmptyStoreDir(t *testing.T) {
	cfg := validSingleNatsConfig()
	cfg.Server.StoreDir = ""

	err := ValidateNatsConfig(cfg, DBSqlite)
	if err == nil {
		t.Fatal("expected error for empty storeDir")
	}
}

func TestProtocolValidate(t *testing.T) {
	tests := []struct {
		name    string
		p       Protocol
		wantErr bool
	}{
		{"json_legacy", Protocol{Encoding: "json", Mode: "legacy"}, false},
		{"json_simple", Protocol{Encoding: "json", Mode: "simple"}, false},
		{"cbor_legacy", Protocol{Encoding: "cbor", Mode: "legacy"}, false},
		{"cbor_simple", Protocol{Encoding: "cbor", Mode: "simple"}, false},
		{"unknown_encoding", Protocol{Encoding: "xml", Mode: "legacy"}, true},
		{"empty_encoding", Protocol{Encoding: "", Mode: "legacy"}, true},
		{"unknown_mode", Protocol{Encoding: "json", Mode: "fast"}, true},
		{"empty_mode", Protocol{Encoding: "json", Mode: ""}, true},
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
