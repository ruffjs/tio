package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
	DefaultEncoding string `json:"defaultEncoding" mapstructure:"defaultEncoding"`
	Mode            string `json:"mode" mapstructure:"mode"`
}

func (p Protocol) Validate() error {
	switch p.DefaultEncoding {
	case "json", "cbor":
	default:
		return fmt.Errorf("protocol.defaultEncoding must be 'json' or 'cbor', got %q", p.DefaultEncoding)
	}
	switch p.Mode {
	case "legacy", "simple", "dual":
	default:
		return fmt.Errorf("protocol.mode must be 'legacy', 'simple' or 'dual', got %q", p.Mode)
	}
	return nil
}

type Config struct {
	Log struct {
		Level string `json:"level,omitzero"`
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
	Auth struct {
		// AllowPasswordBootstrap lets a thing with an empty AuthValue store the
		// first password it presents. Off by default.
		AllowPasswordBootstrap bool `json:"allowPasswordBootstrap"`
	} `json:"auth"`
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
	// Nats is decoded separately from config-nats.default.yaml and the main
	// config's nats block; see loadNats.
	Nats NatsConfig `json:"nats" mapstructure:"-"`
}

func ReadConfig() Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/tio/")
	v.AddConfigPath("$HOME/.tio")
	v.AddConfigPath(".")
	cfg, err := loadConfig(v)
	if err != nil {
		log.Fatalf("Error read config file: %v", err)
	}
	return cfg
}

// LoadConfigFile loads a main config and its optional NATS file.
func LoadConfigFile(filename string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(filename)
	return loadConfig(v)
}

func loadConfig(main *viper.Viper) (Config, error) {
	if err := main.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			return Config{}, fmt.Errorf("read main config: %w", err)
		}
	}
	if main.InConfig("protocol.encoding") {
		return Config{}, fmt.Errorf("rename protocol.encoding to protocol.defaultEncoding; Thing.payloadEncoding takes precedence in every mode")
	}

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(defaultConfigYaml)); err != nil {
		return Config{}, fmt.Errorf("read defaults: %w", err)
	}
	if err := v.MergeConfigMap(main.AllSettings()); err != nil {
		return Config{}, fmt.Errorf("merge main config: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode main config: %w", err)
	}

	if err := loadNats(main, &cfg); err != nil {
		return Config{}, err
	}
	if err := validateMqttBrokerSuperUsers(cfg.Connector.MqttBroker.SuperUsers); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// loadNats decodes the NATS config by merging, in order: the embedded NATS
// defaults, the main config's nats block, then (optionally) the file referenced
// by nats.configFile (resolved relative to the main config file).
func loadNats(main *viper.Viper, cfg *Config) error {
	nats := viper.New()
	nats.SetConfigType("yaml")
	if err := nats.ReadConfig(bytes.NewReader(tio.DefaultNatsConfigYaml)); err != nil {
		return fmt.Errorf("read NATS defaults: %w", err)
	}
	// The defaults are wrapped under a top-level "nats" key, mirroring the main
	// config's nats section.
	if sub := nats.Sub("nats"); sub != nil {
		nats = sub
	} else {
		return fmt.Errorf("NATS defaults missing top-level nats key")
	}

	if sub := main.Sub("nats"); sub != nil {
		if err := nats.MergeConfigMap(sub.AllSettings()); err != nil {
			return fmt.Errorf("merge nats config: %w", err)
		}
	}

	if err := nats.UnmarshalExact(&cfg.Nats); err != nil {
		return fmt.Errorf("decode NATS config: %w", err)
	}

	if filename := cfg.Nats.ConfigFile; filename != "" {
		if !filepath.IsAbs(filename) {
			filename = filepath.Join(filepath.Dir(main.ConfigFileUsed()), filename)
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read NATS config %q: %w", filename, err)
		}
		// The referenced file uses the same wrapped shape as the defaults.
		file := viper.New()
		file.SetConfigType("yaml")
		if err := file.ReadConfig(bytes.NewReader(data)); err != nil {
			return fmt.Errorf("read NATS config %q: %w", filename, err)
		}
		if sub := file.Sub("nats"); sub != nil {
			if err := nats.MergeConfigMap(sub.AllSettings()); err != nil {
				return fmt.Errorf("read NATS config %q: %w", filename, err)
			}
		} else {
			return fmt.Errorf("read NATS config %q: missing top-level nats key", filename)
		}
		if err := nats.UnmarshalExact(&cfg.Nats); err != nil {
			return fmt.Errorf("decode NATS config: %w", err)
		}
		cfg.Nats.ConfigFile = filename
	}

	if cfg.Nats.Enabled {
		if err := ValidateNatsConfig(cfg.Nats, cfg.DB.Typ); err != nil {
			return err
		}
	}
	return nil
}

type Connector struct {
	MqttBroker InnerMqttBroker `json:"mqttBroker"`
}

type NatsConfig struct {
	Enabled      bool                `json:"enabled" mapstructure:"enabled"`
	Mode         string              `json:"mode" mapstructure:"mode"`
	URL          string              `json:"url" mapstructure:"url"`
	ConfigFile   string              `json:"configFile" mapstructure:"configFile"`
	Server       NatsServerConfig    `json:"server" mapstructure:"server"`
	AppClient    NatsClientConfig    `json:"appClient" mapstructure:"appClient"`
	SystemClient NatsClientConfig    `json:"systemClient" mapstructure:"systemClient"`
	JetStream    NatsJetStreamConfig `json:"jetStream" mapstructure:"jetStream"`
}

type NatsJetStreamConfig struct {
	Enabled             bool   `json:"enabled" mapstructure:"enabled"`
	StoreDir            string `json:"storeDir" mapstructure:"storeDir"`
	MaxMemory           int64  `json:"maxMemory" mapstructure:"maxMemory"`
	MaxStore            int64  `json:"maxStore" mapstructure:"maxStore"`
	MaxAgeSeconds       int64  `json:"maxAgeSeconds" mapstructure:"maxAgeSeconds"`
	MaxMsgs             int64  `json:"maxMsgs" mapstructure:"maxMsgs"`
	MaxBytes            int64  `json:"maxBytes" mapstructure:"maxBytes"`
	DefaultMaxConsumers int    `json:"defaultMaxConsumers" mapstructure:"defaultMaxConsumers"`
}

type NatsServerConfig struct {
	ServerName       string        `json:"serverName"`
	ClusterName      string        `json:"clusterName"`
	Port             int           `json:"port"`
	ClusterPort      int           `json:"clusterPort"`
	ClusterAdvertise string        `json:"clusterAdvertise"`
	Routes           []string      `json:"routes"`
	MonitorPort      int           `json:"monitorPort"`
	ClusterUser      string        `json:"clusterUser"`
	ClusterPassword  string        `json:"-"`
	ClientTLS        NatsTLSConfig `json:"clientTls"`
	RouteTLS         NatsTLSConfig `json:"routeTls"`
}

type NatsTLSConfig struct {
	CertFile          string `json:"certFile"`
	KeyFile           string `json:"keyFile"`
	CAFile            string `json:"caFile"`
	RequireClientCert bool   `json:"requireClientCert"`
}

type NatsClientConfig struct {
	User     string              `json:"user"`
	Password string              `json:"-"`
	TLS      NatsTLSClientConfig `json:"tls"`
}

type NatsTLSClientConfig struct {
	CAFile   string `json:"caFile"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

func ValidateNatsConfig(cfg NatsConfig, dbType string) error {
	if !cfg.Enabled {
		return nil
	}

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

	if cfg.JetStream.Enabled && cfg.JetStream.StoreDir == "" {
		return fmt.Errorf("nats jetStream.storeDir must not be empty when JetStream is enabled")
	}
	if cfg.JetStream.DefaultMaxConsumers < -1 {
		return fmt.Errorf("nats jetStream.defaultMaxConsumers must be -1, 0, or positive")
	}

	if !natsTLSEmpty(s.RouteTLS) {
		if s.RouteTLS.CAFile == "" || s.RouteTLS.CertFile == "" || s.RouteTLS.KeyFile == "" {
			return fmt.Errorf("nats routeTls requires caFile, certFile, and keyFile to all be set")
		}
	}

	if cfg.AppClient.User == "" {
		return fmt.Errorf("nats appClient.user must not be empty")
	}
	if cfg.AppClient.Password == "" {
		return fmt.Errorf("nats appClient.password must not be empty")
	}
	if cfg.SystemClient.User == "" {
		return fmt.Errorf("nats systemClient.user must not be empty")
	}
	if cfg.SystemClient.Password == "" {
		return fmt.Errorf("nats systemClient.password must not be empty")
	}

	return nil
}

func natsTLSEmpty(t NatsTLSConfig) bool {
	return t.CertFile == "" && t.KeyFile == "" && t.CAFile == ""
}

func validateMqttBrokerSuperUsers(users []UserPassword) error {
	for _, u := range users {
		if u.Name == "" {
			return fmt.Errorf("connector.mqttBroker.superUsers[].name must not be empty")
		}
		if u.Password == "" {
			return fmt.Errorf("connector.mqttBroker.superUsers[%q].password must not be empty", u.Name)
		}
	}
	return nil
}

var GlobalCtxCancel context.CancelFunc
