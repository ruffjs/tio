package connector

import (
	"log/slog"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"ruff.io/tio/db/mysql"
)

// MySQL connector

const TypeMySQL = "mysql"

func init() {
	Register(TypeMySQL, func(name string, cfg map[string]any) (Conn, error) {
		var ac mysql.Config
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			return nil, errors.WithMessage(err, "decode config")
		}
		c := &MySQL{
			name:   name,
			config: ac,
		}
		c.Connect()
		return c, nil
	})
}

type MySQL struct {
	name   string
	config mysql.Config
	db     *gorm.DB
}

func (c *MySQL) Close() error {
	if d, err := c.db.DB(); err != nil {
		return errors.WithMessage(err, "get db")
	} else {
		return d.Close()
	}
}

func (c *MySQL) Status() Status {
	d, err := c.db.DB()
	if err != nil {
		return StatusDisconnected
	}
	if err := d.Ping(); err != nil {
		return StatusDisconnected
	} else {
		return StatusConnected
	}
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
