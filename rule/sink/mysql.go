package sink

import (
	"log/slog"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
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
