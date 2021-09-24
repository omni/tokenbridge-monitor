package ethclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestResults = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "monitor",
		Subsystem: "rpc",
		Name:      "request_results_total",
	}, []string{"url", "query", "status"})

	RequestDurations = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "monitor",
		Subsystem: "rpc",
		Name:      "request_duration_seconds",
		Buckets:   []float64{0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 20},
	}, []string{"url", "query"})
)

func ObserveError(url, query string, err error) {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			RequestResults.WithLabelValues(url, query, "timeout").Inc()
		} else if err, ok := err.(rpc.Error); ok {
			RequestResults.WithLabelValues(url, query, fmt.Sprintf("error-%d-%s", err.ErrorCode(), err.Error())).Inc()
		} else {
			RequestResults.WithLabelValues(url, query, "error").Inc()
		}
	} else {
		RequestResults.WithLabelValues(url, query, "ok").Inc()
	}
}

func ObserveDuration(url, query string) func() time.Duration {
	return prometheus.NewTimer(RequestDurations.WithLabelValues(url, query)).ObserveDuration
}
