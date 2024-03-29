package alerts

import (
	"context"
	"fmt"
	"time"

	"github.com/omni/tokenbridge-monitor/config"
	"github.com/omni/tokenbridge-monitor/db"
	"github.com/omni/tokenbridge-monitor/logging"
)

type AlertManager struct {
	logger logging.Logger
	jobs   map[string]*Job
}

//nolint:cyclop,funlen
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
		case "unknown_erc_to_native_message_confirmation":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownErcToNativeConfirmations,
				Metric:   NewAlertUnknownErcToNativeMessageConfirmation(cfg.ID),
			}
		case "unknown_erc_to_native_message_execution":
			jobs[name] = &Job{
				Interval: time.Minute,
				Timeout:  time.Second * 10,
				Func:     provider.FindUnknownErcToNativeExecutions,
				Metric:   NewAlertUnknownErcToNativeMessageExecution(cfg.ID),
			}
		case "stuck_erc_to_native_message_confirmation":
			jobs[name] = &Job{
				Interval: time.Minute * 5,
				Timeout:  time.Second * 20,
				Func:     provider.FindStuckErcToNativeMessages,
				Metric:   NewAlertStuckErcToNativeMessageConfirmation(cfg.ID),
			}
		case "last_validator_activity":
			jobs[name] = &Job{
				Interval: time.Minute * 10,
				Timeout:  time.Second * 20,
				Func:     provider.FindLastValidatorActivity,
				Metric:   NewAlertLastValidatorActivity(cfg.ID),
			}
		default:
			return nil, fmt.Errorf("unknown alert type %q: %w", name, config.ErrInvalidConfig)
		}
		jobs[name].Params = &AlertJobParams{
			Bridge:                  cfg.ID,
			HomeChainID:             cfg.Home.Chain.ChainID,
			HomeStartBlockNumber:    alertCfg.HomeStartBlock,
			HomeBridgeAddress:       cfg.Home.Address,
			HomeWhitelistedSenders:  cfg.Home.WhitelistedSenders,
			ForeignChainID:          cfg.Foreign.Chain.ChainID,
			ForeignStartBlockNumber: alertCfg.ForeignStartBlock,
			ForeignBridgeAddress:    cfg.Foreign.Address,
		}
	}

	return &AlertManager{
		logger: logger,
		jobs:   jobs,
	}, nil
}

func (m *AlertManager) Start(ctx context.Context, isSynced func() bool) {
	ticker := time.NewTicker(10 * time.Second)
	for !isSynced() {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			m.logger.Debug("waiting for bridge monitor to be synchronized on both sides")
		}
	}
	ticker.Stop()
	m.logger.Info("both sides of the monitor are synced, starting alert manager jobs")

	for name, job := range m.jobs {
		job.logger = m.logger.WithField("alert_job", name)
		go job.Start(ctx, isSynced)
	}
}
