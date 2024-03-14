package rule

import (
	"github.com/spf13/viper"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
)

type Config struct {
	Connectors []connector.Config
	Sinks      []sink.Config
	Sources    []source.Config
	Rules      []RuleConfig
}

type RuleConfig struct {
	Name    string
	Sources []string
	Process []Process
	Sinks   []string
}

type Process struct {
	Name string
	Type string
	Jq   string
}

type AmqpSinkOption struct {
	Exchange   string
	RoutingKey string
}

type MqttSourceOption struct {
	Topic string
	Qos   byte
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
