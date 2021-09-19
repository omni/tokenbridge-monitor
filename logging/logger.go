package logging

import (
	"time"

	"github.com/sirupsen/logrus"
)

type Logger logrus.FieldLogger

func New() Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	})
	return logger
}
