package rocketmq

import (
	"github.com/582033/gin-utils/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var baselog = &Log{
	LogLevel: log.LoggerLevel(),
	Logger:   log.Logger(),
}

type Log struct {
	LogLevel zapcore.Level
	Logger   *zap.SugaredLogger
}

func (l *Log) OutputPath(path string) (err error) {
	return nil
}

func (l *Log) Level(level string) {

}

func (l *Log) Warning(msg string, fields map[string]interface{}) {
	if l.LogLevel > zapcore.WarnLevel {
		return
	}
	l.Logger.Warn(msg, fields)
}

func (l *Log) Debug(msg string, fields map[string]interface{}) {
	if l.LogLevel > zapcore.DebugLevel {
		return
	}
	l.Logger.Debug(msg, fields)
}

func (l *Log) Error(msg string, fields map[string]interface{}) {
	if l.LogLevel > zapcore.ErrorLevel {
		return
	}
	l.Logger.Error(msg, fields)
}

func (l *Log) Fatal(msg string, fields map[string]interface{}) {
	if l.LogLevel > zapcore.FatalLevel {
		return
	}
	l.Logger.Fatal(msg, fields)
}

func (l *Log) Info(msg string, fields map[string]interface{}) {
	if l.LogLevel > zapcore.InfoLevel {
		return
	}
	l.Logger.Info(msg, fields)
}
