package sink

import (
	"context"
	"fmt"
	"log/slog"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/connector"
	ruleconnector "ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

// MySQL sink, use raw SQL
// Transform data to SQL and sink data to MySQL.

// Example for update or insert latest data properties
//   - input: {
// 							"payload": {
// 								"sn": "wm-liu",
// 								"data": {
// 									"temp": 112,
// 									"hum": 50
// 								}
// 							}
// 						}
//   - jq:      .payload as {sn: $sn, data: $data} | $data
//            		| to_entries
//            		| map(" (\"" + .key + "\", \"" + $sn + "\", NOW(), \"" + (.value | tostring) + "\",\"" + (.value | type)  + "\")")
//            		| join(",")
//  		          | "INSERT INTO `data_latest` (`name`, `sn`, `time`, `value`, `type`) VALUES" + .  + "ON DUPLICATE KEY UPDATE `time` = VALUES(`time`), `value` = VALUES(`value`), `type`=VALUES(`type`)"
//   - output: INSERT INTO `data_latest` (`name`, `sn`, `time`, `value`, `type`) VALUES ("temp", "wm-liu", NOW(), "112","number"), ("hum", "wm-liu", NOW(), "50","number")ON DUPLICATE KEY UPDATE `time` = VALUES(`time`), `value` = VALUES(`value`), `type`=VALUES(`type`)

const TypeMySQL = "mysql"

func init() {
	Register(TypeMySQL, NewMySQL)
}

type MySqlConfig struct {
}

func NewMySQL(ctx context.Context, name string, cfg map[string]any, ruleConn ruleconnector.Conn, _ connector.Connector) (Sink, error) {
	var ac MySqlConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, fmt.Errorf("decode config")
	}
	c, ok := ruleConn.(*ruleconnector.MySQL)
	if !ok {
		return nil, fmt.Errorf("wrong connector type for mysql sink")
	}

	a := &mysqlImpl{
		ctx:  ctx,
		name: name,
		cfg:  ac,
		conn: c,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a, nil
}

type mysqlImpl struct {
	ctx  context.Context
	name string
	cfg  MySqlConfig
	conn *ruleconnector.MySQL
	ch   chan *Msg

	started bool
}

func (s *mysqlImpl) Start() error {
	s.started = true
	slog.Info("Rule start sink", "type", s.Type(), "name", s.name)
	return s.Status().Error
}

func (s *mysqlImpl) Status() model.StatusInfo {
	if !s.started {
		return model.StatusNotStarted()
	}
	return withConnStatus(s.conn.Name(), s.conn.Status())
}

func (s *mysqlImpl) Stop() error {
	s.started = false
	slog.Info("Rule stop sink", "type", s.Type(), "name", s.name)
	return nil
}

func (s *mysqlImpl) Name() string {
	return s.name
}

func (*mysqlImpl) Type() string {
	return TypeMySQL
}

func (s *mysqlImpl) Publish(msg Msg) {
	if s.started {
		s.ch <- &msg
	}
}

func (s *mysqlImpl) publishLoop() {
	for {
		msg := <-s.ch
		sql := string(msg.Payload)
		if res := s.conn.DB().WithContext(s.ctx).Exec(sql); res.Error != nil {
			slog.Error("MySQL sink exec failed", "error", res.Error, "sql", sql)
		} else {
			slog.Debug("MySQL sink exec succeeded", "payload", sql)
		}
	}
}
