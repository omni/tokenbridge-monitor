package db

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var QueryDurations = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "monitor",
	Subsystem: "db",
	Name:      "query_duration_seconds",
	Help:      "Shows database query durations.",
	Buckets:   []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 15, 20},
}, []string{"query"})

func ObserveDuration(query string) func() time.Duration {
	return prometheus.NewTimer(QueryDurations.WithLabelValues(query)).ObserveDuration
}
