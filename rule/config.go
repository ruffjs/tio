package rule

import (
	"log/slog"

	"github.com/spf13/viper"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/process"
	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
)

type Config struct {
	Connectors []connector.Config `json:"connectors"`
	Sinks      []sink.Config      `json:"sinks"`
	Sources    []source.Config    `json:"sources"`
	Rules      []RuleConfig       `json:"rules"`
}

type RuleConfig struct {
	Name    string           `json:"name"`
	Note    string           `json:"note"`
	Enabled bool             `json:"enabled"`
	Sources []string         `json:"sources"`
	Process []process.Config `json:"process"`
	Sinks   []string         `json:"sinks"`
}

type AmqpSinkOption struct {
	Exchange   string
	RoutingKey string
}

type MqttSourceOption struct {
	Topic string `json:"topic"`
	Qos   byte   `json:"qos"`
}

func ReadConfig() (Config, error) {
	v := viper.New()
	v.SetConfigName("config-rule")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/tio/")
	v.AddConfigPath("$HOME/.tio")
	v.AddConfigPath(".")

	err := v.ReadInConfig()
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	err = v.Unmarshal(&cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func SetConfig(cfg Config) error {
	slog.Info("Rule config set", "content", cfg)

	v := viper.New()
	v.SetConfigName("config-rule")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/tio/")
	v.AddConfigPath("$HOME/.tio")
	v.AddConfigPath(".")

	v.Set("connectors", cfg.Connectors)
	v.Set("sinks", cfg.Sinks)
	v.Set("sources", cfg.Sources)
	v.Set("rules", cfg.Rules)

	err := v.WriteConfig()
	if err != nil {
		return err
	}

	return nil
}
