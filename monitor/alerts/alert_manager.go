package alerts

import (
	"amb-monitor/config"
	"amb-monitor/db"
	"amb-monitor/logging"
	"context"
	"fmt"
	"time"
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
				Metric:   NewAlertUnknownMessageConfirmation(cfg.ID),
			}
		case "unknown_message_execution":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownExecutions,
				Metric:   NewAlertUnknownMessageExecution(cfg.ID),
			}
		case "stuck_message_confirmation":
			jobs[name] = &Job{
				Interval: time.Minute * 5,
				Timeout:  time.Second * 20,
				Func:     provider.FindStuckMessages,
				Metric:   NewAlertStuckMessageConfirmation(cfg.ID),
			}
		case "failed_message_execution":
			jobs[name] = &Job{
				Interval: time.Minute * 5,
				Timeout:  time.Second * 20,
				Func:     provider.FindFailedExecutions,
				Metric:   NewAlertFailedMessageExecution(cfg.ID),
			}
		case "unknown_information_signature":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownInformationSignatures,
				Metric:   NewAlertUnknownInformationSignature(cfg.ID),
			}
		case "unknown_information_execution":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownInformationExecutions,
				Metric:   NewAlertUnknownInformationExecution(cfg.ID),
			}
		case "stuck_information_request":
			jobs[name] = &Job{
				Interval: time.Minute * 5,
				Timeout:  time.Second * 20,
				Func:     provider.FindStuckInformationRequests,
				Metric:   NewAlertStuckInformationRequest(cfg.ID),
			}
		case "failed_information_request":
			jobs[name] = &Job{
				Interval: time.Minute * 5,
				Timeout:  time.Second * 20,
				Func:     provider.FindFailedInformationRequests,
				Metric:   NewAlertFailedInformationRequest(cfg.ID),
			}
		case "different_information_signatures":
			jobs[name] = &Job{
				Interval: time.Minute * 5,
				Timeout:  time.Second * 10,
				Func:     provider.FindDifferentInformationSignatures,
				Metric:   NewAlertDifferentInformationSignatures(cfg.ID),
			}
		default:
			return nil, fmt.Errorf("unknown alert type %q", name)
		}
		jobs[name].Params = &AlertJobParams{
			Bridge:                  cfg.ID,
			HomeChainID:             cfg.Home.Chain.ChainID,
			HomeStartBlockNumber:    cfg.Home.StartBlock,
			HomeBridgeAddress:       cfg.Home.Address,
			ForeignChainID:          cfg.Foreign.Chain.ChainID,
			ForeignStartBlockNumber: cfg.Foreign.StartBlock,
			ForeignBridgeAddress:    cfg.Foreign.Address,
		}
		if alertCfg != nil {
			if alertCfg.HomeStartBlock > 0 {
				jobs[name].Params.HomeStartBlockNumber = alertCfg.HomeStartBlock
			}
			if alertCfg.ForeignStartBlock > 0 {
				jobs[name].Params.ForeignStartBlockNumber = alertCfg.ForeignStartBlock
			}
		}
	}

	return &AlertManager{
		logger: logger,
		jobs:   jobs,
	}, nil
}

func (m *AlertManager) Start(ctx context.Context, isSynced func() bool) {
	t := time.NewTicker(10 * time.Second)
	for !isSynced() {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			m.logger.Debug("waiting for bridge monitor to be synchronized on both sides")
		}
	}
	t.Stop()
	m.logger.Info("both sides of the monitor are synced, starting alert manager jobs")

	for name, job := range m.jobs {
		job.logger = m.logger.WithField("alert_job", name)
		go job.Start(ctx, isSynced)
	}
}
