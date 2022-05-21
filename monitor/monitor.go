package monitor

import (
	"context"
	"fmt"
	"tokenbridge-monitor/config"
	"tokenbridge-monitor/contract/abi"
	"tokenbridge-monitor/db"
	"tokenbridge-monitor/ethclient"
	"tokenbridge-monitor/logging"
	"tokenbridge-monitor/monitor/alerts"
	"tokenbridge-monitor/repository"
)

type Monitor struct {
	cfg            *config.BridgeConfig
	logger         logging.Logger
	repo           *repository.Repo
	homeMonitor    *ContractMonitor
	foreignMonitor *ContractMonitor
	alertManager   *alerts.AlertManager
}

func NewMonitor(ctx context.Context, logger logging.Logger, dbConn *db.DB, repo *repository.Repo, cfg *config.BridgeConfig, homeClient, foreignClient ethclient.Client) (*Monitor, error) {
	logger.Info("initializing bridge monitor")
	homeMonitor, err := NewContractMonitor(ctx, logger.WithField("contract", "home"), repo, cfg, cfg.Home, homeClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize home side monitor: %w", err)
	}
	foreignMonitor, err := NewContractMonitor(ctx, logger.WithField("contract", "foreign"), repo, cfg, cfg.Foreign, foreignClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize foreign side monitor: %w", err)
	}
	alertManager, err := alerts.NewAlertManager(logger, dbConn, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize alert manager: %w", err)
	}
	monitor := &Monitor{
		cfg:            cfg,
		logger:         logger,
		repo:           repo,
		homeMonitor:    homeMonitor,
		foreignMonitor: foreignMonitor,
		alertManager:   alertManager,
	}
	switch cfg.BridgeMode {
	case config.BridgeModeErcToNative:
		monitor.RegisterErcToNativeEventHandlers()
	case config.BridgeModeArbitraryMessage:
		monitor.RegisterAMBEventHandlers()
	}
	err = monitor.homeMonitor.VerifyEventHandlersABI()
	if err != nil {
		return nil, fmt.Errorf("home side contract does not have %s event in its ABI", err)
	}
	err = monitor.foreignMonitor.VerifyEventHandlersABI()
	if err != nil {
		return nil, fmt.Errorf("foreign side contract does not have %s event in its ABI", err)
	}
	return monitor, nil
}

func (m *Monitor) RegisterErcToNativeEventHandlers() {
	handlers := NewBridgeEventHandler(m.repo, m.cfg, m.homeMonitor.client)
	m.homeMonitor.RegisterEventHandler(abi.ErcToNativeUserRequestForSignature, handlers.HandleErcToNativeUserRequestForSignature)
	m.homeMonitor.RegisterEventHandler(abi.SignedForUserRequest, handlers.HandleSignedForUserRequest)
	m.homeMonitor.RegisterEventHandler(abi.CollectedSignatures, handlers.HandleCollectedSignatures)
	m.homeMonitor.RegisterEventHandler(abi.ErcToNativeSignedForAffirmation, handlers.HandleErcToNativeSignedForAffirmation)
	m.homeMonitor.RegisterEventHandler(abi.ErcToNativeAffirmationCompleted, handlers.HandleErcToNativeAffirmationCompleted)
	m.homeMonitor.RegisterEventHandler(abi.ValidatorAdded, handlers.HandleValidatorAdded)
	m.homeMonitor.RegisterEventHandler(abi.ValidatorRemoved, handlers.HandleValidatorRemoved)

	m.foreignMonitor.RegisterEventHandler(abi.ErcToNativeUserRequestForAffirmation, handlers.HandleErcToNativeUserRequestForAffirmation)
	m.foreignMonitor.RegisterEventHandler(abi.ErcToNativeTransfer, handlers.HandleErcToNativeTransfer)
	m.foreignMonitor.RegisterEventHandler(abi.ErcToNativeRelayedMessage, handlers.HandleErcToNativeRelayedMessage)
	m.foreignMonitor.RegisterEventHandler(abi.ValidatorAdded, handlers.HandleValidatorAdded)
	m.foreignMonitor.RegisterEventHandler(abi.ValidatorRemoved, handlers.HandleValidatorRemoved)
}

func (m *Monitor) RegisterAMBEventHandlers() {
	handlers := NewBridgeEventHandler(m.repo, m.cfg, m.homeMonitor.client)
	m.homeMonitor.RegisterEventHandler(abi.UserRequestForSignature, handlers.HandleUserRequestForSignature)
	m.homeMonitor.RegisterEventHandler(abi.LegacyUserRequestForSignature, handlers.HandleLegacyUserRequestForSignature)
	m.homeMonitor.RegisterEventHandler(abi.SignedForUserRequest, handlers.HandleSignedForUserRequest)
	m.homeMonitor.RegisterEventHandler(abi.CollectedSignatures, handlers.HandleCollectedSignatures)
	m.homeMonitor.RegisterEventHandler(abi.SignedForAffirmation, handlers.HandleSignedForUserRequest)
	m.homeMonitor.RegisterEventHandler(abi.AffirmationCompleted, handlers.HandleAffirmationCompleted)
	m.homeMonitor.RegisterEventHandler(abi.LegacyAffirmationCompleted, handlers.HandleAffirmationCompleted)
	m.homeMonitor.RegisterEventHandler(abi.UserRequestForInformation, handlers.HandleUserRequestForInformation)
	m.homeMonitor.RegisterEventHandler(abi.SignedForInformation, handlers.HandleSignedForInformation)
	m.homeMonitor.RegisterEventHandler(abi.InformationRetrieved, handlers.HandleInformationRetrieved)
	m.homeMonitor.RegisterEventHandler(abi.ValidatorAdded, handlers.HandleValidatorAdded)
	m.homeMonitor.RegisterEventHandler(abi.ValidatorRemoved, handlers.HandleValidatorRemoved)

	m.foreignMonitor.RegisterEventHandler(abi.UserRequestForAffirmation, handlers.HandleUserRequestForAffirmation)
	m.foreignMonitor.RegisterEventHandler(abi.LegacyUserRequestForAffirmation, handlers.HandleLegacyUserRequestForAffirmation)
	m.foreignMonitor.RegisterEventHandler(abi.RelayedMessage, handlers.HandleRelayedMessage)
	m.foreignMonitor.RegisterEventHandler(abi.LegacyRelayedMessage, handlers.HandleRelayedMessage)
	m.foreignMonitor.RegisterEventHandler(abi.ValidatorAdded, handlers.HandleValidatorAdded)
	m.foreignMonitor.RegisterEventHandler(abi.ValidatorRemoved, handlers.HandleValidatorRemoved)
}

func (m *Monitor) Start(ctx context.Context) {
	m.logger.Info("starting bridge monitor")
	go m.homeMonitor.Start(ctx)
	go m.foreignMonitor.Start(ctx)
	go m.alertManager.Start(ctx, m.IsSynced)
}

func (m *Monitor) IsSynced() bool {
	return m.homeMonitor.IsSynced() && m.foreignMonitor.IsSynced()
}
