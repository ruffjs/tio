package config

import (
	"bytes"

	"ruff.io/tio"
	"ruff.io/tio/pkg/log"

	_ "embed"

	"github.com/spf13/viper"
	"ruff.io/tio/db/mysql"
	"ruff.io/tio/db/sqlite"
)

var (
	Version   = ""
	GitCommit = ""

	defaultConfigYaml []byte
)

func init() {
	defaultConfigYaml = tio.DefaultConfigYaml
}

const (
	// DBSqlite DB type
	DBSqlite = "sqlite"
	DBMySQL  = "mysql"

	// Connector type

	// ConnectorEmqx EMQX MQTT broker
	ConnectorEmqx = "emqx"
	// ConnectorMqttEmbed MQTT broker embedded in tio
	ConnectorMqttEmbed = "embed"
)

type UserPassword struct {
	Name     string `json:"name"`
	Password string `json:"-"`
}

type Redis struct {
	Addr      string `json:"redis"`
	Password  string `json:"-"`
	DB        int    `json:"db"`
	KeyPrefix string `json:"keyPrefix"`
}

type InnerMqttStorage struct {
	Type     string `json:"type"`
	FilePath string `json:"filePath"`
	Redis    Redis  `json:"redis"`
}

type InnerMqttBroker struct {
	TcpPort          int              `json:"tcpPort"`
	TcpSslPort       int              `json:"tcpSslPort"`
	WsPort           int              `json:"wsPort"`
	WssPort          int              `json:"wssPort"`
	PublicTcpPort    *int             `json:"publicTcpPort"`
	PublicTcpSslPort *int             `json:"publicTcpSslPort"`
	PublicWsPort     *int             `json:"publicWsPort"`
	PublicWssPort    *int             `json:"publicWssPort"`
	CertFile         string           `json:"-"`
	KeyFile          string           `json:"-"`
	Storage          InnerMqttStorage `json:"storage"`
	SuperUsers       []UserPassword   `json:"superUsers"`
}

type Config struct {
	Log log.Config `json:"log"`
	API struct {
		Port      int          `json:"port"`
		Cors      bool         `json:"cors"`
		BasicAuth UserPassword `json:"basicAuth"`
	} `json:"api"`
	DB struct {
		Typ    string        `json:"type" mapstructure:"type"`
		Mysql  mysql.Config  `json:"mysql"`
		Sqlite sqlite.Config `json:"sqlite"`
	} `json:"db"`
	Connector Connector `json:"connector"`
}

func ReadConfig() Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/tio/")
	viper.AddConfigPath("$HOME/.tio")
	viper.AddConfigPath(".")

	err := viper.ReadConfig(bytes.NewReader(defaultConfigYaml))
	if err != nil {
		log.Fatalf("Error read default config file: %v", err)
	}

	err = viper.MergeInConfig()
	if err != nil {
		log.Fatalf("Error read config file: %v", err)
	}
	var cfg Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Fatalf("Error read config file content: %v", err)
	}
	return cfg
}

type Connector struct {
	Typ        string            `json:"type" mapstructure:"type"`
	MqttClient MqttClientConfig  `json:"mqttClient"`
	MqttBroker InnerMqttBroker   `json:"mqttBroker"`
	Emqx       EmqxAdapterConfig `json:"emqx"`
}

type MqttClientConfig struct {
	ClientId     string `json:"clientId"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"-"`
	WillTopic    string `json:"WillTopic"`
	WillPayload  string `json:"willPayload"`
	CleanSession *bool  `json:"cleanSession"`
}

type EmqxAdapterConfig struct {
	ApiPrefix   string `json:"apiPrefix"` // like http://localhost:18083
	ApiUser     string `json:"apiUser"`
	ApiPassword string `json:"-"`
}
