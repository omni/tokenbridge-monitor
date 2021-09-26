package alerts

import (
	"amb-monitor/config"
	"amb-monitor/db"
	"amb-monitor/logging"
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type AlertManager struct {
	logger logging.Logger
	jobs   map[string]*Job
}

func NewAlertManager(logger logging.Logger, db *db.DB, cfg *config.BridgeConfig) (*AlertManager, error) {
	provider := NewDBAlertsProvider(db)
	jobs := make(map[string]*Job, len(cfg.Alerts))

	bridgeLabel := prometheus.Labels{
		"bridge_id": cfg.ID,
	}
	for name, alertCfg := range cfg.Alerts {
		switch name {
		case "unknown_message_confirmation":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownConfirmations,
				Metric:   AlertUnknownMessageConfirmation.MustCurryWith(bridgeLabel),
			}
		case "unknown_message_execution":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownExecutions,
				Metric:   AlertUnknownMessageExecution.MustCurryWith(bridgeLabel),
			}
		case "stuck_message_confirmation":
			jobs[name] = &Job{
				Interval: time.Minute * 2,
				Timeout:  time.Second * 20,
				Func:     provider.FindStuckMessages,
				Metric:   AlertStuckMessageConfirmation.MustCurryWith(bridgeLabel),
			}
		case "failed_message_execution":
			jobs[name] = &Job{
				Interval: time.Minute * 5,
				Timeout:  time.Second * 20,
				Func:     provider.FindFailedExecutions,
				Metric:   AlertFailedMessageExecution.MustCurryWith(bridgeLabel),
			}
		default:
			return nil, fmt.Errorf("unknown alert type %q", name)
		}
		jobs[name].ResetMetric = func() { jobs[name].Metric.Delete(bridgeLabel) }
		jobs[name].Params = &AlertJobParams{
			Bridge:                  cfg.ID,
			HomeChainID:             cfg.Home.Chain.ChainID,
			HomeStartBlockNumber:    alertCfg.HomeStartBlock,
			ForeignChainID:          cfg.Foreign.Chain.ChainID,
			ForeignStartBlockNumber: alertCfg.ForeignStartBlock,
		}
	}

	return &AlertManager{
		logger: logger,
		jobs:   jobs,
	}, nil
}

func (m *AlertManager) Start(ctx context.Context) {
	for name, job := range m.jobs {
		job.logger = m.logger.WithField("alert_job", name)
		go job.Start(ctx)
	}
}
