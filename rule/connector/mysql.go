package connector

import (
	"context"
	"log/slog"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"ruff.io/tio/db/mysql"
	"ruff.io/tio/rule/model"
)

// MySQL connector

const TypeMySQL = "mysql"

func init() {
	Register(TypeMySQL, func(ctx context.Context, name string, cfg map[string]any) (Conn, error) {
		var ac mysql.Config
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			return nil, errors.WithMessage(err, "decode config")
		}
		c := &MySQL{
			name:   name,
			config: ac,
		}
		return c, nil
	})
}

type MySQL struct {
	name   string
	config mysql.Config
	db     *gorm.DB

	started bool
	status  model.StatusInfo
}

func (c *MySQL) Start() error {
	if c.started {
		return nil
	}
	c.started = true

	db, err := mysql.Connect(c.config)
	if err != nil {
		slog.Error("MySQL connect db", "error", err)
		c.status = model.StatusDisconnected("connect: "+err.Error(), err)
		return err
	}
	c.db = db
	return nil
}

func (c *MySQL) Stop() error {
	c.started = false
	if c.db == nil {
		return nil
	}
	if d, err := c.db.DB(); err != nil {
		return errors.WithMessage(err, "get db")
	} else {
		return d.Close()
	}
}

func (c *MySQL) Status() model.StatusInfo {
	if !c.started {
		return model.StatusNotStarted()
	}
	if c.db == nil {
		return c.status
	}

	d, err := c.db.DB()
	if err != nil {
		return model.StatusDisconnected(err.Error(), err)
	}
	if err := d.Ping(); err != nil {
		return model.StatusDisconnected("ping db: "+err.Error(), err)
	} else {
		return model.StatusConnected()
	}
}

func (c *MySQL) Name() string {
	return c.name
}

func (*MySQL) Type() string {
	return TypeMySQL
}

func (c *MySQL) DB() *gorm.DB {
	return c.db
}
