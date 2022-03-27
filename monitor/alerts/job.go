package alerts

import (
	"amb-monitor/logging"
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type AlertValues struct {
	Labels map[string]string
	Value  float64
}

type AlertJobParams struct {
	Bridge                  string
	HomeChainID             string
	HomeStartBlockNumber    uint
	HomeBridgeAddress       common.Address
	HomeWhitelistedSenders  []common.Address
	ForeignChainID          string
	ForeignStartBlockNumber uint
	ForeignBridgeAddress    common.Address
}

type Job struct {
	logger   logging.Logger
	Metric   *prometheus.GaugeVec
	Interval time.Duration
	Timeout  time.Duration
	Func     func(ctx context.Context, params *AlertJobParams) ([]AlertValues, error)
	Params   *AlertJobParams
}

func (j *Job) Start(ctx context.Context, isSynced func() bool) {
	ticker := time.NewTicker(j.Interval)
	for {
		if isSynced() {
			timeoutCtx, cancel := context.WithTimeout(ctx, j.Timeout)
			start := time.Now()
			values, err := j.Func(timeoutCtx, j.Params)
			cancel()
			if err != nil {
				j.logger.WithError(err).Error("failed to process alert job")
			} else {
				j.Metric.Reset()

				if len(values) > 0 {
					j.logger.WithFields(logrus.Fields{
						"count":    len(values),
						"duration": time.Since(start),
					}).Warn("found some possible alerts")
					for _, v := range values {
						j.Metric.With(v.Labels).Set(v.Value)
					}
				} else {
					j.logger.WithField("duration", time.Since(start)).Info("no alerts has been found")
				}
			}
		} else {
			j.logger.Warn("bridge monitor is not synchronized, skipping alert job iteration")
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
