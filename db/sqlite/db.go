package sqlite

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm/logger"

	"gorm.io/gorm"
)

type PoolConfig struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}
type Config struct {
	FilePath string
	ShowSql  bool
}

type loggerImp struct{}

func (l *loggerImp) Printf(format string, args ...interface{}) {
	slog.Info(fmt.Sprintf(format, args...))
}

func Connect(cfg Config) (*gorm.DB, error) {
	logLevel := logger.Info
	if !cfg.ShowSql {
		logLevel = logger.Silent
	}
	l := logger.New(
		&loggerImp{},
		logger.Config{
			SlowThreshold:             time.Millisecond * 200, // Slow SQL threshold
			LogLevel:                  logLevel,               // Log level
			IgnoreRecordNotFoundError: true,                   // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,                  // Disable color
		},
	)
	db, err := gorm.Open(
		sqlite.Open(cfg.FilePath),
		&gorm.Config{Logger: l},
	)
	if err != nil {
		return nil, err
	}

	// set connection pool size to 1
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(1)

	return db, nil
}
