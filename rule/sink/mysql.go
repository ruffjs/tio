package sink

import (
	"log/slog"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
)

// AMQP sink for message forward

const TypeMySQL = "mysql"

func init() {
	Register(TypeMySQL, NewMySQL)
}

type MySqlConfig struct {
}

func NewMySQL(name string, cfg map[string]any, conn connector.Conn) Sink {
	var ac MySqlConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("decode sink mysql config", "name", name, "error", err)
		os.Exit(1)
	}
	c, ok := conn.(*connector.MySQL)
	if !ok {
		slog.Error("wrong connector type for mysql sink")
		os.Exit(1)
	}

	a := &mysqlImpl{
		name: name,
		cfg:  ac,
		conn: c,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a
}

type mysqlImpl struct {
	name string
	cfg  MySqlConfig
	conn *connector.MySQL
	ch   chan *Msg
}

func (s *mysqlImpl) Name() string {
	return s.name
}

func (*mysqlImpl) Type() string {
	return TypeMySQL
}

func (s *mysqlImpl) Publish(msg Msg) {
	s.ch <- &msg
}

func (s *mysqlImpl) publishLoop() {
	for {
		msg := <-s.ch
		sql := string(msg.Payload)
		if res := s.conn.DB().Exec(sql); res.Error != nil {
			slog.Error("MySQL sink exec failed", "error", res.Error, "sql", sql)
		}
	}
}
