package logging

import (
	"context"
	"io"
	"time"

	"github.com/sirupsen/logrus"
)

type ctxKey int

const loggerCtxKey ctxKey = iota

type Logger logrus.FieldLogger

func New() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	})
	return logger
}

func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, logger)
}

func NullLogger() Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func LoggerFromContext(ctx context.Context) Logger {
	logger, ok := ctx.Value(loggerCtxKey).(Logger)
	if ok {
		return logger
	}
	return NullLogger()
}
