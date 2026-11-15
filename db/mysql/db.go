package mysql

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Host            string `json:"host"`
	Port            string `json:"port"`
	User            string `json:"user"`
	Password        string `json:"-"`
	DB              string `json:"db"`
	Charset         string `json:"charset"`
	Timezone        string `json:"timezone"`
	MaxIdleConns    int    `json:"maxIdleConns"`
	MaxOpenConns    int    `json:"maxOpenConns"`
	ConnMaxLifetime int    `json:"connMaxLifetime"` // seconds
	ShowSql         bool   `json:"showSql"`
}

type loggerImp struct{}

func (l *loggerImp) Printf(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

func dsn(cfg Config, withReadTimeout bool) string {
	s := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&loc=%s&parseTime=True&timeout=5s&writeTimeout=10s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DB, cfg.Charset, cfg.Timezone,
	)
	if withReadTimeout {
		s += "&readTimeout=10s"
	}
	return s
}

// Connect returns the business connection pool. Reads are bounded so a hung
// query cannot hold a handler goroutine forever.
func Connect(cfg Config) (*gorm.DB, error) {
	return connect(cfg, true)
}

// ConnectForMigration returns a connection without readTimeout: auto migration
// runs DDL, which may legitimately take longer than a normal read.
func ConnectForMigration(cfg Config) (*gorm.DB, error) {
	return connect(cfg, false)
}

func connect(cfg Config, withReadTimeout bool) (*gorm.DB, error) {
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
			DSN:               dsn(cfg, withReadTimeout),
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
