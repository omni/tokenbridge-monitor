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
		Help:      "Shows counter for different RPC query results.",
	}, []string{"chain_id", "url", "query", "status"})

	RequestDurations = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "monitor",
		Subsystem: "rpc",
		Name:      "request_duration_seconds",
		Help:      "Shows RPC query durations.",
		Buckets:   []float64{0.05, 0.1, 0.2, 0.5, 1, 2, 4, 6, 8, 10, 12, 15, 20},
	}, []string{"chain_id", "url", "query"})
)

func ObserveError(chainID, url, query string, err error) {
	result := "ok"
	if err != nil {
		var rpcErr rpc.Error
		result = "error"
		if errors.Is(err, context.DeadlineExceeded) {
			result = "timeout"
		} else if errors.As(err, &rpcErr) {
			result = fmt.Sprintf("error-%d-%s", rpcErr.ErrorCode(), rpcErr.Error())
		}
	}
	RequestResults.WithLabelValues(chainID, url, query, result).Inc()
}

func ObserveDuration(chainID, url, query string) func() time.Duration {
	return prometheus.NewTimer(RequestDurations.WithLabelValues(chainID, url, query)).ObserveDuration
}
