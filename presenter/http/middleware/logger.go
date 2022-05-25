package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"

	"github.com/poanetwork/tokenbridge-monitor/logging"
)

func NewLoggerMiddleware(logger logging.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			logger = logger.WithFields(logrus.Fields{
				"request_id":  middleware.GetReqID(ctx),
				"http_method": r.Method,
				"http_path":   r.RequestURI,
			})
			ctx = logging.WithLogger(ctx, logger)

			logger.Info("handling http request")
			ts := time.Now()
			next.ServeHTTP(w, r.WithContext(ctx))
			logger.WithField("duration", time.Since(ts)).Info("http request completed")
		})
	}
}
