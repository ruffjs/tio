package connector

import (
	"log/slog"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mitchellh/mapstructure"
	"gorm.io/gorm"
	"ruff.io/tio/db/mysql"
)

// MySQL connector

const TypeMySQL = "mysql"

func init() {
	Register(TypeMySQL, func(name string, cfg map[string]any) Conn {
		var ac mysql.Config
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			slog.Error("Failed to decode config", "error", err)
			os.Exit(1)
		}
		c := &MySQL{
			name:   name,
			config: ac,
		}
		c.Connect()
		return c
	})
}

type MySQL struct {
	name   string
	config mysql.Config
	db     *gorm.DB
}

func (c *MySQL) Status() Status {
	panic("unimplemented")
}

func (c *MySQL) Name() string {
	return c.name
}

func (*MySQL) Type() string {
	return TypeMySQL
}

func (c *MySQL) Connect() error {
	db, err := mysql.Connect(c.config)
	if err != nil {
		slog.Error("MySQL connect db", "error", err)
		return err
	}
	c.db = db
	return nil
}

func (c *MySQL) DB() *gorm.DB {
	return c.db
}
