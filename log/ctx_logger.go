package log

import (
	"context"
	"fmt"
	"github.com/582033/gin-utils/ctx"
	"go.uber.org/zap"
)

type ContextLogger struct {
	ctx context.Context
}

func WithCtx(ctx context.Context) *ContextLogger {
	return &ContextLogger{
		ctx: ctx,
	}
}

func (l *ContextLogger) Debug(args ...interface{}) {
	log.With(addContext(l.ctx)).Debug(args...)
}

func (l *ContextLogger) Debugf(template string, args ...interface{}) {
	log.With(addContext(l.ctx)).Debugf(template, args...)
}

func (l *ContextLogger) Println(args ...interface{}) {
	log.With(addContext(l.ctx)).Info(args...)
}

func (l *ContextLogger) Printf(template string, args ...interface{}) {
	log.With(addContext(l.ctx)).Infof(template, args...)
}

func (l *ContextLogger) Info(args ...interface{}) {
	log.With(addContext(l.ctx)).Info(args...)
}

func (l *ContextLogger) Infof(template string, args ...interface{}) {
	log.With(addContext(l.ctx)).Infof(template, args...)
}

func (l *ContextLogger) Warn(args ...interface{}) {
	log.With(addContext(l.ctx)).Warn(args...)
}

func (l *ContextLogger) Warnf(template string, args ...interface{}) {
	log.With(addContext(l.ctx)).Warnf(template, args...)
}

func (l *ContextLogger) Error(args ...interface{}) {
	log.With(addContext(l.ctx)).Error(args...)
}

func (l *ContextLogger) Errorf(template string, args ...interface{}) {
	log.With(addContext(l.ctx)).Errorf(template, args...)
}

func (l *ContextLogger) Panic(args ...interface{}) {
	log.With(addContext(l.ctx)).Panic(args...)
}

func (l *ContextLogger) Panicf(template string, args ...interface{}) {
	log.With(addContext(l.ctx)).Panicf(template, args...)
}

func (l *ContextLogger) Fatal(args ...interface{}) {
	log.With(addContext(l.ctx)).Fatal(args...)
}

func (l *ContextLogger) Fatalf(template string, args ...interface{}) {
	log.With(addContext(l.ctx)).Fatalf(template, args...)
}

func addContext(c context.Context) zap.Field {
	return zap.String("tid", fmt.Sprintf("%v", c.Value(ctx.BaseContextRequestIDKey)))
}
