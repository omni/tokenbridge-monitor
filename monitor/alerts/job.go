package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"tokenbridge-monitor/logging"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

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

type AlertMetricValues map[string]string

const ValueLabelTag = "_value"

func (v AlertMetricValues) Labels() prometheus.Labels {
	labels := make(prometheus.Labels, len(v))
	for k, val := range v {
		if k != ValueLabelTag {
			labels[k] = val
		}
	}
	return labels
}

func (v AlertMetricValues) Value() float64 {
	val, ok := v[ValueLabelTag]
	if !ok {
		return 0
	}
	res, _ := strconv.ParseFloat(val, 64)
	return res
}

func ConvertToAlertMetricValues(v interface{}) ([]AlertMetricValues, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("can't marshal alert values to json: %w", err)
	}
	res := make([]AlertMetricValues, 10)
	err = json.Unmarshal(raw, &res)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal alert values to []AlertMetricValues: %w", err)
	}
	return res, nil
}

type Job struct {
	logger   logging.Logger
	Metric   *prometheus.GaugeVec
	Interval time.Duration
	Timeout  time.Duration
	Func     func(ctx context.Context, params *AlertJobParams) (interface{}, error)
	Params   *AlertJobParams
}

func (j *Job) Start(ctx context.Context, isSynced func() bool) {
	ticker := time.NewTicker(j.Interval)
	for {
		if isSynced() {
			timeoutCtx, cancel := context.WithTimeout(ctx, j.Timeout)
			start := time.Now()
			alerts, err := j.Func(timeoutCtx, j.Params)
			cancel()
			if err != nil {
				j.logger.WithError(err).Error("failed to process alert job")
			} else {
				j.Metric.Reset()
				values, err2 := ConvertToAlertMetricValues(alerts)
				if err2 != nil {
					j.logger.WithError(err2).Error("can't convert to alert metric values")
				} else if len(values) > 0 {
					j.logger.WithFields(logrus.Fields{
						"count":    len(values),
						"duration": time.Since(start),
					}).Warn("found some possible alerts")
					for _, v := range values {
						j.Metric.With(v.Labels()).Set(v.Value())
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
