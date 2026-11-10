package integration_tests

import (
	"log"

	"ruff.io/tio/config"
)

func ReadConfig() config.Config {
	cfg, err := config.LoadConfigFile("config-test.yaml")
	if err != nil {
		log.Fatalf("Error read config file: %v", err)
	}
	return cfg
}
