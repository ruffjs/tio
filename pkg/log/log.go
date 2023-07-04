package log

import (
	"github.com/kpango/glg"
)

func init() {
	glg.Get().SetCallerDepth(3)
}

func Debug(val ...interface{}) {
	_ = glg.Debug(val...)
}

func Debugf(format string, val ...interface{}) {
	err := glg.Debugf(format, val...)
	if err != nil {
		return
	}
}

func Info(val ...interface{}) {
	_ = glg.Info(val...)
}

func Infof(format string, val ...interface{}) {
	_ = glg.Infof(format, val...)
}

func Warn(val ...interface{}) {
	_ = glg.Warn(val...)
}

func Warnf(format string, val ...interface{}) {
	_ = glg.Warnf(format, val...)
}

func Error(val ...interface{}) {
	_ = glg.Error(val...)
}

func Errorf(format string, val ...interface{}) {
	_ = glg.Errorf(format, val...)
}

func Fatal(val ...interface{}) {
	glg.Fatal(val...)
}

func Fatalf(format string, val ...interface{}) {
	glg.Fatalf(format, val...)
}
