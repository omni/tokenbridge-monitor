package middleware

import (
	"net/http"

	"github.com/omni/tokenbridge-monitor/logging"
)

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger := logging.LoggerFromContext(r.Context())
				if err2, ok := err.(error); ok {
					logger = logger.WithError(err2)
				} else {
					logger = logger.WithField("recovered", err)
				}
				logger.Error("recovered error from the http handler")
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
