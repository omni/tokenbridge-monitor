package presenter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
	"github.com/poanetwork/tokenbridge-monitor/logging"
	"github.com/poanetwork/tokenbridge-monitor/presenter/http/middleware"
	"github.com/poanetwork/tokenbridge-monitor/presenter/http/render"
	"github.com/poanetwork/tokenbridge-monitor/repository"
)

type Presenter struct {
	logger logging.Logger
	repo   *repository.Repo
	cfg    *config.Config
	root   chi.Router
}

func NewPresenter(logger logging.Logger, repo *repository.Repo, cfg *config.Config) *Presenter {
	return &Presenter{
		logger: logger,
		repo:   repo,
		cfg:    cfg,
		root:   chi.NewMux(),
	}
}

func (p *Presenter) Serve(addr string) error {
	p.logger.WithField("addr", addr).Info("starting presenter service")
	p.root.Use(chimiddleware.Throttle(5))
	p.root.Use(chimiddleware.RequestID)
	p.root.Use(middleware.NewLoggerMiddleware(p.logger))
	p.root.Use(middleware.Recoverer)
	registerSearchRoutes := func(r chi.Router) {
		r.Use(middleware.GetFilterMiddleware)
		r.Get("/", p.GetMessages)
		r.Get("/logs", p.GetLogs)
		r.Get("/messages", p.GetMessages)
	}
	p.root.Route("/bridge/{bridgeID:[0-9a-zA-Z_\\-]+}", func(r chi.Router) {
		r.Use(middleware.GetBridgeConfigMiddleware(p.cfg))
		r.Get("/", p.GetBridgeInfo)
		r.Get("/info", p.GetBridgeInfo)
		r.Get("/config", p.GetBridgeConfig)
		r.Get("/validators", p.GetBridgeValidators)
	})
	p.root.Route("/chain/{chainID:[0-9]+}", func(r chi.Router) {
		r.Use(middleware.GetChainConfigMiddleware(p.cfg))
		r.Route("/block/{blockNumber:[0-9]+}", func(r2 chi.Router) {
			r2.Use(middleware.GetBlockNumberMiddleware)
			r2.Group(registerSearchRoutes)
		})
		r.Route("/tx/{txHash:0x[0-9a-fA-F]{64}}", func(r2 chi.Router) {
			r2.Use(middleware.GetTxHashMiddleware)
			r2.Group(registerSearchRoutes)
		})
	})
	p.root.Route("/tx/{txHash:0x[0-9a-fA-F]{64}}", func(r chi.Router) {
		r.Use(middleware.GetTxHashMiddleware)
		r.Group(registerSearchRoutes)
	})
	return http.ListenAndServe(addr, p.root)
}

func (p *Presenter) getBridgeSideInfo(ctx context.Context, bridgeID string, cfg *config.BridgeSideConfig) (*BridgeSideInfo, error) {
	cursor, err := p.repo.LogsCursors.GetByChainIDAndAddress(ctx, cfg.Chain.ChainID, cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get home bridge cursor: %w", err)
	}

	var lastFetchedBlockTime, lastProcessedBlockTime time.Time
	bt, err := p.repo.BlockTimestamps.GetByBlockNumber(ctx, cfg.Chain.ChainID, cursor.LastFetchedBlock)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, fmt.Errorf("failed to get home bridge cursor: %w", err)
	} else if err == nil {
		lastFetchedBlockTime = bt.Timestamp
	}

	if cursor.LastFetchedBlock == cursor.LastProcessedBlock {
		lastProcessedBlockTime = lastFetchedBlockTime
	} else {
		bt, err = p.repo.BlockTimestamps.GetByBlockNumber(ctx, cfg.Chain.ChainID, cursor.LastProcessedBlock)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			return nil, fmt.Errorf("failed to get home bridge cursor: %w", err)
		} else if err == nil {
			lastProcessedBlockTime = bt.Timestamp
		}
	}

	validators, err := p.repo.BridgeValidators.FindActiveValidators(ctx, bridgeID, cfg.Chain.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to find validators for bridge id: %w", err)
	}
	validatorAddresses := make([]common.Address, len(validators))
	for i, v := range validators {
		validatorAddresses[i] = v.Address
	}

	return &BridgeSideInfo{
		Chain:                  cfg.ChainName,
		ChainID:                cfg.Chain.ChainID,
		BridgeAddress:          cfg.Address,
		LastFetchedBlock:       cursor.LastFetchedBlock,
		LastFetchBlockTime:     lastFetchedBlockTime,
		LastProcessedBlock:     cursor.LastProcessedBlock,
		LastProcessedBlockTime: lastProcessedBlockTime,
		Validators:             validatorAddresses,
	}, nil
}

func (p *Presenter) GetBridgeConfig(w http.ResponseWriter, r *http.Request) {
	cfg := middleware.BridgeConfig(r.Context())

	render.JSON(w, r, http.StatusOK, cfg)
}

func (p *Presenter) GetBridgeInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := middleware.BridgeConfig(ctx)

	homeInfo, err := p.getBridgeSideInfo(ctx, cfg.ID, cfg.Home)
	if err != nil {
		render.Error(w, r, fmt.Errorf("failed to get home bridge info: %w", err))
		return
	}
	foreignInfo, err := p.getBridgeSideInfo(ctx, cfg.ID, cfg.Foreign)
	if err != nil {
		render.Error(w, r, fmt.Errorf("failed to get foreign bridge info: %w", err))
		return
	}

	render.JSON(w, r, http.StatusOK, &BridgeInfo{
		BridgeID: cfg.ID,
		Mode:     cfg.BridgeMode,
		Home:     homeInfo,
		Foreign:  foreignInfo,
	})
}

func (p *Presenter) GetBridgeValidators(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := middleware.BridgeConfig(ctx)

	homeValidators, err := p.repo.BridgeValidators.FindActiveValidators(ctx, cfg.ID, cfg.Home.Chain.ChainID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("failed to find home validators: %w", err))
		return
	}

	foreignValidators, err := p.repo.BridgeValidators.FindActiveValidators(ctx, cfg.ID, cfg.Home.Chain.ChainID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("failed to find home validators: %w", err))
		return
	}

	//nolint:gocritic
	validators := append(homeValidators, foreignValidators...)
	res := &ValidatorsInfo{
		BridgeID: cfg.ID,
		Mode:     cfg.BridgeMode,
	}

	seenValidators := make(map[common.Address]bool, len(validators))
	for _, val := range validators {
		if seenValidators[val.Address] {
			continue
		}
		seenValidators[val.Address] = true

		valInfo := &ValidatorInfo{
			Address: val.Address,
		}
		confirmation, err2 := p.repo.SignedMessages.FindLatest(ctx, cfg.ID, cfg.Home.Chain.ChainID, val.Address)
		if err2 != nil {
			if !errors.Is(err, db.ErrNotFound) {
				render.Error(w, r, fmt.Errorf("failed to find latest validator confirmation: %w", err))
				return
			}
		} else {
			valInfo.LastConfirmation, err = p.getTxInfo(ctx, confirmation.LogID)
			if err != nil {
				render.Error(w, r, fmt.Errorf("failed to get tx info: %w", err))
				return
			}
		}
		res.Validators = append(res.Validators, valInfo)
	}

	render.JSON(w, r, http.StatusOK, res)
}

func (p *Presenter) getFilteredLogs(ctx context.Context) ([]*entity.Log, error) {
	filter := middleware.GetFilterContext(ctx)

	if filter.TxHash == nil {
		if filter.ChainID == nil {
			return nil, errors.New("chainId query parameter is missing")
		}
		if filter.FromBlock == nil || filter.ToBlock == nil {
			return nil, errors.New("block query parameters are missing")
		}
	}
	return p.repo.Logs.Find(ctx, entity.LogsFilter{
		ChainID:   filter.ChainID,
		FromBlock: filter.FromBlock,
		ToBlock:   filter.ToBlock,
		TxHash:    filter.TxHash,
	})
}

func (p *Presenter) GetMessages(w http.ResponseWriter, r *http.Request) {
	logs, err := p.getFilteredLogs(r.Context())
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't filter logs: %w", err))
		return
	}

	res := p.searchForMessagesInLogs(r.Context(), logs)
	render.JSON(w, r, http.StatusOK, res)
}

func (p *Presenter) GetLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := p.getFilteredLogs(r.Context())
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't filter logs: %w", err))
		return
	}

	res := make([]*LogResult, len(logs))
	for i, log := range logs {
		res[i] = &LogResult{
			LogID:       log.ID,
			ChainID:     log.ChainID,
			Address:     log.Address,
			Topic0:      log.Topic0,
			Topic1:      log.Topic1,
			Topic2:      log.Topic2,
			Topic3:      log.Topic3,
			Data:        log.Data,
			TxHash:      log.TransactionHash,
			BlockNumber: log.BlockNumber,
		}
	}

	render.JSON(w, r, http.StatusOK, res)
}

func (p *Presenter) searchForMessagesInLogs(ctx context.Context, logs []*entity.Log) []*SearchResult {
	results := make([]*SearchResult, 0, len(logs))
	for _, log := range logs {
		for _, task := range []func(context.Context, *entity.Log) (*SearchResult, error){
			p.searchSentMessage,
			p.searchSignedMessage,
			p.searchExecutedMessage,
			p.searchSentInformationRequest,
			p.searchSignedInformationRequest,
			p.searchExecutedInformationRequest,
		} {
			if res, err := task(ctx, log); err != nil && !errors.Is(err, db.ErrNotFound) {
				p.logger.WithError(err).Error("failed to execute search task")
			} else if res != nil {
				for _, e := range res.RelatedEvents {
					if e.LogID == log.ID {
						res.Event = e
						break
					}
				}
				if res.Event == nil {
					p.logger.Error("tx event not found in related events")
				}
				results = append(results, res)
				break
			}
		}
	}
	return results
}

func (p *Presenter) searchSentMessage(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	sent, err := p.repo.SentMessages.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForMessage(ctx, sent.BridgeID, &sent.MsgHash, nil)
}

func (p *Presenter) searchSignedMessage(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	sig, err := p.repo.SignedMessages.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForMessage(ctx, sig.BridgeID, &sig.MsgHash, nil)
}

func (p *Presenter) searchExecutedMessage(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	executed, err := p.repo.ExecutedMessages.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForMessage(ctx, executed.BridgeID, nil, &executed.MessageID)
}

func (p *Presenter) buildSearchResultForMessage(ctx context.Context, bridgeID string, msgHash, messageID *common.Hash) (*SearchResult, error) {
	if msgHash == nil && messageID == nil {
		return nil, errors.New("msgHash and messageID can't be both nil")
	}
	var messageInfo interface{}
	var events []*EventInfo
	var msg *entity.Message
	var err error
	var searchID common.Hash
	if msgHash != nil {
		searchID = *msgHash
		msg, err = p.repo.Messages.FindByMsgHash(ctx, bridgeID, *msgHash)
	} else {
		searchID = *messageID
		msg, err = p.repo.Messages.FindByMessageID(ctx, bridgeID, *messageID)
	}
	if err != nil && errors.Is(err, db.ErrNotFound) {
		ercToNativeMsg, err2 := p.repo.ErcToNativeMessages.FindByMsgHash(ctx, bridgeID, searchID)
		if err2 != nil {
			return nil, err2
		}
		messageInfo = NewErcToNativeMessageInfo(ercToNativeMsg)
		events, err = p.buildMessageEvents(ctx, ercToNativeMsg.BridgeID, ercToNativeMsg.MsgHash, ercToNativeMsg.MsgHash)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		messageInfo = NewMessageInfo(msg)
		events, err = p.buildMessageEvents(ctx, msg.BridgeID, msg.MsgHash, msg.MessageID)
		if err != nil {
			return nil, err
		}
	}
	return &SearchResult{
		Message:       messageInfo,
		RelatedEvents: events,
	}, nil
}

func (p *Presenter) searchSentInformationRequest(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	sent, err := p.repo.SentInformationRequests.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForInformationRequest(ctx, sent.BridgeID, sent.MessageID)
}

func (p *Presenter) searchSignedInformationRequest(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	signed, err := p.repo.SignedInformationRequests.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForInformationRequest(ctx, signed.BridgeID, signed.MessageID)
}

func (p *Presenter) searchExecutedInformationRequest(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	executed, err := p.repo.ExecutedInformationRequests.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForInformationRequest(ctx, executed.BridgeID, executed.MessageID)
}

func (p *Presenter) buildSearchResultForInformationRequest(ctx context.Context, bridgeID string, messageID common.Hash) (*SearchResult, error) {
	req, err := p.repo.InformationRequests.FindByMessageID(ctx, bridgeID, messageID)
	if err != nil {
		return nil, err
	}
	events, err := p.buildInformationRequestEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Message:       NewInformationRequestInfo(req),
		RelatedEvents: events,
	}, nil
}

func (p *Presenter) buildMessageEvents(ctx context.Context, bridgeID string, msgHash, messageID common.Hash) ([]*EventInfo, error) {
	sent, err := p.repo.SentMessages.FindByMsgHash(ctx, bridgeID, msgHash)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}
	signed, err := p.repo.SignedMessages.FindByMsgHash(ctx, bridgeID, msgHash)
	if err != nil {
		return nil, err
	}
	collected, err := p.repo.CollectedMessages.FindByMsgHash(ctx, bridgeID, msgHash)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}
	executed, err := p.repo.ExecutedMessages.FindByMessageID(ctx, bridgeID, messageID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}

	events := make([]*EventInfo, 0, 5)
	if sent != nil {
		events = append(events, &EventInfo{
			Action: "SENT_MESSAGE",
			LogID:  sent.LogID,
		})
	}
	for _, s := range signed {
		events = append(events, &EventInfo{
			Action: "SIGNED_MESSAGE",
			LogID:  s.LogID,
			Signer: &s.Signer,
		})
	}
	if collected != nil {
		events = append(events, &EventInfo{
			Action: "COLLECTED_SIGNATURES",
			LogID:  collected.LogID,
			Count:  collected.NumSignatures,
		})
	}
	if executed != nil {
		events = append(events, &EventInfo{
			Action: "EXECUTED_MESSAGE",
			LogID:  executed.LogID,
			Status: executed.Status,
		})
	}
	return p.enrichEvents(ctx, events)
}

func (p *Presenter) buildInformationRequestEvents(ctx context.Context, req *entity.InformationRequest) ([]*EventInfo, error) {
	sent, err := p.repo.SentInformationRequests.FindByMessageID(ctx, req.BridgeID, req.MessageID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}
	signed, err := p.repo.SignedInformationRequests.FindByMessageID(ctx, req.BridgeID, req.MessageID)
	if err != nil {
		return nil, err
	}
	executed, err := p.repo.ExecutedInformationRequests.FindByMessageID(ctx, req.BridgeID, req.MessageID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}

	events := make([]*EventInfo, 0, 5)
	if sent != nil {
		events = append(events, &EventInfo{
			Action: "SENT_INFORMATION_REQUEST",
			LogID:  sent.LogID,
		})
	}
	for _, s := range signed {
		events = append(events, &EventInfo{
			Action: "SIGNED_INFORMATION_REQUEST",
			LogID:  s.LogID,
			Signer: &s.Signer,
			Data:   s.Data,
		})
	}
	if executed != nil {
		events = append(events, &EventInfo{
			Action:         "EXECUTED_INFORMATION_REQUEST",
			LogID:          executed.LogID,
			Status:         executed.Status,
			CallbackStatus: executed.CallbackStatus,
		})
	}
	return p.enrichEvents(ctx, events)
}

func (p *Presenter) enrichEvents(ctx context.Context, events []*EventInfo) ([]*EventInfo, error) {
	var err error
	for _, event := range events {
		event.TxInfo, err = p.getTxInfo(ctx, event.LogID)
		if err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (p *Presenter) getTxInfo(ctx context.Context, logID uint) (*TxInfo, error) {
	log, err := p.repo.Logs.GetByID(ctx, logID)
	if err != nil {
		return nil, err
	}
	bt, err := p.repo.BlockTimestamps.GetByBlockNumber(ctx, log.ChainID, log.BlockNumber)
	if err != nil {
		return nil, err
	}
	return &TxInfo{
		BlockNumber: log.BlockNumber,
		Timestamp:   bt.Timestamp,
		Link:        FormatLogTxLinkURL(log),
	}, nil
}
