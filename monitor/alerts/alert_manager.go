package alerts

import (
	"amb-monitor/config"
	"amb-monitor/db"
	"amb-monitor/logging"
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type AlertManager struct {
	logger logging.Logger
	jobs   map[string]*Job
}

func NewAlertManager(logger logging.Logger, db *db.DB, cfg *config.BridgeConfig) (*AlertManager, error) {
	provider := NewDBAlertsProvider(db)
	jobs := make(map[string]*Job, len(cfg.Alerts))

	for name, alertCfg := range cfg.Alerts {
		switch name {
		case "unknown_message_confirmation":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownConfirmations,
				Labels:   []string{"chain_id", "block_number", "tx_hash", "signer", "msg_hash"},
			}
		case "unknown_message_execution":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownExecutions,
				Labels:   []string{"chain_id", "block_number", "tx_hash", "message_id"},
			}
		case "stuck_message_confirmation":
			jobs[name] = &Job{
				Interval: time.Minute * 2,
				Timeout:  time.Second * 20,
				Func:     provider.FindStuckMessages,
				Labels:   []string{"chain_id", "block_number", "tx_hash", "msg_hash", "count"},
			}
		case "failed_message_execution":
			jobs[name] = &Job{
				Interval: time.Minute * 5,
				Timeout:  time.Second * 20,
				Func:     provider.FindFailedExecutions,
				Labels:   []string{"chain_id", "block_number", "tx_hash", "sender", "executor"},
			}
		default:
			return nil, fmt.Errorf("unknown alert type %q", name)
		}
		jobs[name].Metric = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "alert",
			Subsystem: "monitor",
			Name:      name,
			ConstLabels: map[string]string{
				"bridge": cfg.ID,
			},
		}, jobs[name].Labels)
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
