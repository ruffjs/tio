package log

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level string
}

var logLevel = map[string]slog.Level{
	"DEBUG": slog.LevelDebug,
	"INFO":  slog.LevelInfo,
	"WARN":  slog.LevelWarn,
	"ERROR": slog.LevelError,
}

var defaultLogger *slog.Logger

func Init(c Config) {
	l, ok := logLevel[strings.ToUpper(c.Level)]
	if !ok {
		panic("Wrong log level config: " + c.Level)
	}

	defaultLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l}))

	slog.SetDefault(defaultLogger)
}

// Deprecated: use slog directly
func Debug(val string) {
	slog.Debug(val)
}

// Deprecated: use slog directly
func Debugf(format string, val ...interface{}) {
	slog.Debug(fmt.Sprintf(format, val...))
}

// Deprecated: use slog directly
func Info(val string) {
	slog.Info(val)
}

// Deprecated: use slog directly
func Infof(format string, val ...interface{}) {
	slog.Info(fmt.Sprintf(format, val...))
}

// Deprecated: use slog directly
func Warn(val string) {
	slog.Warn(val)
}

// Deprecated: use slog directly
func Warnf(format string, val ...interface{}) {
	slog.Warn(fmt.Sprintf(format, val...))
}

// Deprecated: use slog directly
func Error(val string) {
	slog.Error(val)
}

// Deprecated: use slog directly
func Errorf(format string, val ...interface{}) {
	slog.Error(fmt.Sprintf(format, val...))
}

// Deprecated: use slog directly
func Fatal(val string) {
	slog.Error(val)
	os.Exit(1)
}

// Deprecated: use slog directly
func Fatalf(format string, val ...interface{}) {
	slog.Error(fmt.Sprintf(format, val...))
	os.Exit(1)
}
