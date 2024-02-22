package mysql

import (
	"fmt"
	"log/slog"
	"time"

	"ruff.io/tio/pkg/log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Host            string
	Port            string
	User            string
	Password        string
	DB              string
	Charset         string
	Timezone        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime int // seconds
	ShowSql         bool
}

type loggerImp struct{}

func (l *loggerImp) Printf(format string, args ...interface{}) {
	slog.Info(fmt.Sprintf(format, args...))
}

func Connect(cfg Config) (*gorm.DB, error) {
	// gorm:gorm@tcp(127.0.0.1:3306)/gorm?charset=utf8&parseTime=True&loc=Local
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&loc=%s&parseTime=True",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DB, cfg.Charset, cfg.Timezone,
	)
	log.Debugf("mysql dsn: %s", dsn)
	logLevel := logger.Info
	if !cfg.ShowSql {
		logLevel = logger.Silent
	}
	l := logger.New(
		&loggerImp{},
		logger.Config{
			SlowThreshold:             time.Millisecond * 100, // Slow SQL threshold
			LogLevel:                  logLevel,               // Log level
			IgnoreRecordNotFoundError: true,                   // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,                  // Disable color
		},
	)
	db, err := gorm.Open(
		mysql.New(mysql.Config{
			DSN:               dsn,
			DefaultStringSize: 256,
		}),
		&gorm.Config{Logger: l},
	)
	if err != nil {
		return nil, err
	}

	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxIdleConns(cfg.MaxIdleConns)
	pool.SetMaxOpenConns(cfg.MaxOpenConns)
	pool.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	return db, nil
}
