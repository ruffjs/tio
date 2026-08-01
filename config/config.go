package config

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"ruff.io/tio"

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
	// ConnectorNats NATS connector
	ConnectorNats = "nats"
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
	TcpPort           int              `json:"tcpPort"`
	TcpSslPort        int              `json:"tcpSslPort"`
	WsPort            int              `json:"wsPort"`
	WssPort           int              `json:"wssPort"`
	PublicTcpPort     *int             `json:"publicTcpPort"`
	PublicTcpSslPort  *int             `json:"publicTcpSslPort"`
	PublicWsPort      *int             `json:"publicWsPort"`
	PublicWssPort     *int             `json:"publicWssPort"`
	CertFile          string           `json:"-" mapstructure:"certFile"`
	KeyFile           string           `json:"-" mapstructure:"keyFile"`
	ClientCAFile      string           `json:"-" mapstructure:"clientCaFile"`
	RequireClientCert bool             `json:"requireClientCert" mapstructure:"requireClientCert"`
	Storage           InnerMqttStorage `json:"storage"`
	SuperUsers        []UserPassword   `json:"superUsers"`
	MaximumInflight   uint16           `json:"maximumInflight"`
}

type Protocol struct {
	Encoding string `json:"encoding" mapstructure:"encoding"`
	Mode     string `json:"mode" mapstructure:"mode"`
}

func (p Protocol) Validate() error {
	switch p.Encoding {
	case "json", "cbor":
	default:
		return fmt.Errorf("protocol.encoding must be 'json' or 'cbor', got %q", p.Encoding)
	}
	switch p.Mode {
	case "legacy", "simple":
	default:
		return fmt.Errorf("protocol.mode must be 'legacy' or 'simple', got %q", p.Mode)
	}
	return nil
}

type Config struct {
	Log struct {
		Level string `json:"level,omitempty"`
	} `json:"log"`
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
	Connector       Connector `json:"connector"`
	Protocol        Protocol  `json:"protocol"`
	ProvisionSecret string    `json:"-"`
	Shadow          struct {
		IgnoreMetadataFor []string `json:"ignoreMetadataFor"`
	} `json:"shadow"`
	Pprof struct {
		Port    int  `json:"port"`
		Enabled bool `json:"enabled"`
	} `json:"pprof"`
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
	Nats       NatsConfig        `json:"nats"`
}

type MqttClientConfig struct {
	ClientId     string     `json:"clientId"`
	Host         string     `json:"host"`
	Port         int        `json:"port"`
	User         string     `json:"user"`
	Password     string     `json:"-"`
	WillTopic    string     `json:"WillTopic"`
	WillPayload  string     `json:"willPayload"`
	CleanSession *bool      `json:"cleanSession"`
	TLS          *TLSConfig `json:"tls,omitempty"`
}

type TLSConfig struct {
	CAFile             string `json:"caFile" mapstructure:"caFile"`
	CertFile           string `json:"certFile" mapstructure:"certFile"`
	KeyFile            string `json:"keyFile" mapstructure:"keyFile"`
	ServerName         string `json:"serverName" mapstructure:"serverName"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify" mapstructure:"insecureSkipVerify"`
}

type EmqxAdapterConfig struct {
	ApiPrefix   string `json:"apiPrefix"` // like http://localhost:18083
	ApiUser     string `json:"apiUser"`
	ApiPassword string `json:"-"`
}

type NatsConfig struct {
	Server        NatsServerConfig `json:"server"`
	AppClient     NatsClientConfig `json:"appClient"`
	SystemClient  NatsClientConfig `json:"systemClient"`
	MqttPublisher NatsClientConfig `json:"mqttPublisher"`
	SuperUsers    []UserPassword   `json:"superUsers"`
}

type NatsServerConfig struct {
	ServerName         string        `json:"serverName"`
	ClusterName        string        `json:"clusterName"`
	Port               int           `json:"port"`
	ClusterPort        int           `json:"clusterPort"`
	ClusterAdvertise   string        `json:"clusterAdvertise"`
	Routes             []string      `json:"routes"`
	MqttPort           int           `json:"mqttPort"`
	WsPort             int           `json:"wsPort"`
	MonitorPort        int           `json:"monitorPort"`
	StoreDir           string        `json:"storeDir"`
	JetStreamMaxMemory int64         `json:"jetStreamMaxMemory"`
	JetStreamMaxStore  int64         `json:"jetStreamMaxStore"`
	MqttStreamReplicas int           `json:"mqttStreamReplicas"`
	PresenceReplicas   int           `json:"presenceReplicas"`
	ClusterUser        string        `json:"clusterUser"`
	ClusterPassword    string        `json:"-"`
	ClientTLS          NatsTLSConfig `json:"clientTls"`
	MqttTLS            NatsTLSConfig `json:"mqttTls"`
	WebsocketTLS       NatsTLSConfig `json:"websocketTls"`
	RouteTLS           NatsTLSConfig `json:"routeTls"`
}

type NatsTLSConfig struct {
	CertFile          string `json:"certFile"`
	KeyFile           string `json:"keyFile"`
	CAFile            string `json:"caFile"`
	RequireClientCert bool   `json:"requireClientCert"`
}

type NatsClientConfig struct {
	User     string             `json:"user"`
	Password string             `json:"-"`
	TLS      NatsTLSClientConfig `json:"tls"`
}

type NatsTLSClientConfig struct {
	CAFile   string `json:"caFile"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

func ValidateNatsConfig(cfg NatsConfig, dbType string) error {
	s := cfg.Server
	isMultiNode := len(s.Routes) > 0

	if s.ServerName == "" {
		return fmt.Errorf("nats server.serverName must not be empty")
	}
	if s.ClusterName == "" {
		return fmt.Errorf("nats server.clusterName must not be empty")
	}

	if isMultiNode && dbType == DBSqlite {
		return fmt.Errorf("nats multi-node routes are not supported with sqlite database")
	}

	if s.StoreDir == "" {
		return fmt.Errorf("nats server.storeDir must not be empty when JetStream is used")
	}

	if !isMultiNode {
		if s.MqttStreamReplicas != 1 {
			return fmt.Errorf("nats single-node mqttStreamReplicas must be 1, got %d", s.MqttStreamReplicas)
		}
		if s.PresenceReplicas != 1 {
			return fmt.Errorf("nats single-node presenceReplicas must be 1, got %d", s.PresenceReplicas)
		}
	} else {
		if s.MqttStreamReplicas != 3 {
			return fmt.Errorf("nats multi-node mqttStreamReplicas must be 3, got %d", s.MqttStreamReplicas)
		}
		if s.PresenceReplicas != 3 {
			return fmt.Errorf("nats multi-node presenceReplicas must be 3, got %d", s.PresenceReplicas)
		}
	}

	if s.MqttPort > 0 && natsTLSEmpty(s.MqttTLS) == false {
		return fmt.Errorf("nats cannot have both plaintext mqttPort and mqttTls configured")
	}

	if !natsTLSEmpty(s.RouteTLS) {
		if s.RouteTLS.CAFile == "" || s.RouteTLS.CertFile == "" || s.RouteTLS.KeyFile == "" {
			return fmt.Errorf("nats routeTls requires caFile, certFile, and keyFile to all be set")
		}
	}

	if cfg.AppClient.User == "" {
		return fmt.Errorf("nats appClient.user must not be empty")
	}
	if cfg.SystemClient.User == "" {
		return fmt.Errorf("nats systemClient.user must not be empty")
	}
	if cfg.MqttPublisher.User == "" {
		return fmt.Errorf("nats mqttPublisher.user must not be empty")
	}

	return nil
}

func natsTLSEmpty(t NatsTLSConfig) bool {
	return t.CertFile == "" && t.KeyFile == "" && t.CAFile == ""
}

var GlobalCtxCancel context.CancelFunc
