package integration_tests

import (
	"log"

	"github.com/spf13/viper"
	"ruff.io/tio/config"
)

func ReadConfig() config.Config {
	viper.SetConfigName("config-test")
	viper.AddConfigPath("./")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error read config file: %v", err)
	}
	var cfg config.Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Fatalf("Error read config file content: %v", err)
	}
	return cfg
}
