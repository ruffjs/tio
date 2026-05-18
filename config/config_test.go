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
