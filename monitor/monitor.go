package monitor

import (
	"context"
	"fmt"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/contract/bridgeabi"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
	"github.com/poanetwork/tokenbridge-monitor/logging"
	"github.com/poanetwork/tokenbridge-monitor/monitor/alerts"
	"github.com/poanetwork/tokenbridge-monitor/repository"
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
		return nil, fmt.Errorf("home side contract does not ABI for registered event handler: %w", err)
	}
	err = monitor.foreignMonitor.VerifyEventHandlersABI()
	if err != nil {
		return nil, fmt.Errorf("foreign side contract does not ABI for registered event handler: %w", err)
	}
	return monitor, nil
}

func (m *Monitor) RegisterErcToNativeEventHandlers() {
	handlers := NewBridgeEventHandler(m.repo, m.cfg, m.homeMonitor.client)
	m.homeMonitor.RegisterEventHandler(bridgeabi.ErcToNativeUserRequestForSignature, handlers.HandleErcToNativeUserRequestForSignature)
	m.homeMonitor.RegisterEventHandler(bridgeabi.SignedForUserRequest, handlers.HandleSignedForUserRequest)
	m.homeMonitor.RegisterEventHandler(bridgeabi.CollectedSignatures, handlers.HandleCollectedSignatures)
	m.homeMonitor.RegisterEventHandler(bridgeabi.ErcToNativeSignedForAffirmation, handlers.HandleErcToNativeSignedForAffirmation)
	m.homeMonitor.RegisterEventHandler(bridgeabi.ErcToNativeAffirmationCompleted, handlers.HandleErcToNativeAffirmationCompleted)
	m.homeMonitor.RegisterEventHandler(bridgeabi.ValidatorAdded, handlers.HandleValidatorAdded)
	m.homeMonitor.RegisterEventHandler(bridgeabi.ValidatorRemoved, handlers.HandleValidatorRemoved)

	m.foreignMonitor.RegisterEventHandler(bridgeabi.ErcToNativeUserRequestForAffirmation, handlers.HandleErcToNativeUserRequestForAffirmation)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.ErcToNativeTransfer, handlers.HandleErcToNativeTransfer)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.ErcToNativeRelayedMessage, handlers.HandleErcToNativeRelayedMessage)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.ValidatorAdded, handlers.HandleValidatorAdded)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.ValidatorRemoved, handlers.HandleValidatorRemoved)
}

func (m *Monitor) RegisterAMBEventHandlers() {
	handlers := NewBridgeEventHandler(m.repo, m.cfg, m.homeMonitor.client)
	m.homeMonitor.RegisterEventHandler(bridgeabi.UserRequestForSignature, handlers.HandleUserRequestForSignature)
	m.homeMonitor.RegisterEventHandler(bridgeabi.LegacyUserRequestForSignature, handlers.HandleLegacyUserRequestForSignature)
	m.homeMonitor.RegisterEventHandler(bridgeabi.SignedForUserRequest, handlers.HandleSignedForUserRequest)
	m.homeMonitor.RegisterEventHandler(bridgeabi.CollectedSignatures, handlers.HandleCollectedSignatures)
	m.homeMonitor.RegisterEventHandler(bridgeabi.SignedForAffirmation, handlers.HandleSignedForUserRequest)
	m.homeMonitor.RegisterEventHandler(bridgeabi.AffirmationCompleted, handlers.HandleAffirmationCompleted)
	m.homeMonitor.RegisterEventHandler(bridgeabi.LegacyAffirmationCompleted, handlers.HandleAffirmationCompleted)
	m.homeMonitor.RegisterEventHandler(bridgeabi.UserRequestForInformation, handlers.HandleUserRequestForInformation)
	m.homeMonitor.RegisterEventHandler(bridgeabi.SignedForInformation, handlers.HandleSignedForInformation)
	m.homeMonitor.RegisterEventHandler(bridgeabi.InformationRetrieved, handlers.HandleInformationRetrieved)
	m.homeMonitor.RegisterEventHandler(bridgeabi.ValidatorAdded, handlers.HandleValidatorAdded)
	m.homeMonitor.RegisterEventHandler(bridgeabi.ValidatorRemoved, handlers.HandleValidatorRemoved)

	m.foreignMonitor.RegisterEventHandler(bridgeabi.UserRequestForAffirmation, handlers.HandleUserRequestForAffirmation)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.LegacyUserRequestForAffirmation, handlers.HandleLegacyUserRequestForAffirmation)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.RelayedMessage, handlers.HandleRelayedMessage)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.LegacyRelayedMessage, handlers.HandleRelayedMessage)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.ValidatorAdded, handlers.HandleValidatorAdded)
	m.foreignMonitor.RegisterEventHandler(bridgeabi.ValidatorRemoved, handlers.HandleValidatorRemoved)
}

func (m *Monitor) Start(ctx context.Context) {
	m.logger.Info("starting bridge monitor")
	go m.homeMonitor.Start(ctx)
	go m.foreignMonitor.Start(ctx)
	go m.alertManager.Start(ctx, m.IsSynced)
}

func (m *Monitor) ProcessBlockRange(ctx context.Context, home bool, fromBlock, toBlock uint) error {
	if home {
		return m.homeMonitor.ProcessBlockRange(ctx, fromBlock, toBlock)
	}
	return m.foreignMonitor.ProcessBlockRange(ctx, fromBlock, toBlock)
}

func (m *Monitor) IsSynced() bool {
	return m.homeMonitor.IsSynced() && m.foreignMonitor.IsSynced()
}
