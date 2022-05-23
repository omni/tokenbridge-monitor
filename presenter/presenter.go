package presenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
	"github.com/poanetwork/tokenbridge-monitor/logging"
	"github.com/poanetwork/tokenbridge-monitor/repository"
)

type ctxKey int

const (
	BridgeCfgCtxKey ctxKey = iota
	ChainCfgCtxKey
	BlockNumberCtxKey
	TxHashCtxKey
	FilterCtxKey
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
	p.root.Use(middleware.Throttle(5))
	p.root.Use(middleware.RequestID)
	p.root.Use(NewRequestLogger(p.logger))
	registerSearchRoutes := func(r chi.Router) {
		r.Use(p.GetFilterMiddleware)
		r.Get("/", p.GetMessages)
		r.Get("/logs", p.GetLogs)
		r.Get("/messages", p.GetMessages)
	}
	p.root.Route("/bridge/{bridgeID:[0-9a-zA-Z_\\-]+}", func(r chi.Router) {
		r.Use(p.GetBridgeConfigMiddleware)
		r.Get("/", p.GetBridgeInfo)
		r.Get("/info", p.GetBridgeInfo)
		r.Get("/config", p.GetBridgeConfig)
		r.Get("/validators", p.GetBridgeValidators)
	})
	p.root.Route("/chain/{chainID:[0-9]+}", func(r chi.Router) {
		r.Use(p.GetChainConfigMiddleware)
		r.Route("/block/{blockNumber:[0-9]+}", func(r2 chi.Router) {
			r2.Use(p.GetBlockNumberMiddleware)
			r2.Group(registerSearchRoutes)
		})
		r.Route("/tx/{txHash:0x[0-9a-fA-F]{64}}", func(r2 chi.Router) {
			r2.Use(p.GetTxHashMiddleware)
			r2.Group(registerSearchRoutes)
		})
	})
	p.root.Route("/tx/{txHash:0x[0-9a-fA-F]{64}}", func(r chi.Router) {
		r.Use(p.GetTxHashMiddleware)
		r.Group(registerSearchRoutes)
	})
	return http.ListenAndServe(addr, p.root)
}

func (p *Presenter) JSON(w http.ResponseWriter, r *http.Request, status int, res interface{}) {
	enc := json.NewEncoder(w)

	if pretty, _ := strconv.ParseBool(chi.URLParam(r, "pretty")); pretty {
		enc.SetIndent("", "  ")
	}

	w.WriteHeader(status)
	if err := enc.Encode(res); err != nil {
		p.Error(w, r, fmt.Errorf("failed to marshal JSON result: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
}

func (p *Presenter) Error(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_, err = w.Write([]byte(err.Error()))
	p.logger.WithError(err).Error("request handling failed")
	if err != nil {
		p.logger.WithError(err).Error("can't write error response")
	}
}

func (p *Presenter) GetBridgeConfigMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bridgeID := chi.URLParam(r, "bridgeID")

		cfg, ok := p.cfg.Bridges[bridgeID]
		if !ok || cfg == nil {
			p.JSON(w, r, http.StatusNotFound, fmt.Sprintf("bridge with id %s not found", bridgeID))
			return
		}

		ctx := context.WithValue(r.Context(), BridgeCfgCtxKey, cfg)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *Presenter) GetChainConfigMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chainID := chi.URLParam(r, "chainID")

		var cfg *config.ChainConfig
		for _, chainCfg := range p.cfg.Chains {
			if chainCfg.ChainID == chainID {
				cfg = chainCfg
				break
			}
		}
		if cfg == nil {
			p.JSON(w, r, http.StatusNotFound, fmt.Sprintf("chain with id %s not found", chainID))
			return
		}

		ctx := context.WithValue(r.Context(), ChainCfgCtxKey, cfg)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *Presenter) GetBlockNumberMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blockNumber, err := strconv.ParseUint(chi.URLParam(r, "blockNumber"), 10, 32)
		if err != nil {
			p.Error(w, r, fmt.Errorf("failed to parse blockNumber: %w", err))
			return
		}

		ctx := context.WithValue(r.Context(), BlockNumberCtxKey, uint(blockNumber))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *Presenter) GetTxHashMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txHash := chi.URLParam(r, "txHash")

		ctx := context.WithValue(r.Context(), TxHashCtxKey, common.HexToHash(txHash))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *Presenter) GetFilterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		filter := &FilterContext{}

		if cfg, ok := ctx.Value(ChainCfgCtxKey).(*config.ChainConfig); ok {
			filter.ChainID = &cfg.ChainID
		}
		if blockNumber, ok := ctx.Value(BlockNumberCtxKey).(uint); ok {
			filter.FromBlock = &blockNumber
			filter.ToBlock = &blockNumber
		}
		if txHash, ok := ctx.Value(TxHashCtxKey).(common.Hash); ok {
			filter.TxHash = &txHash
		}

		ctx = context.WithValue(ctx, FilterCtxKey, filter)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
	ctx := r.Context()
	cfg, _ := ctx.Value(BridgeCfgCtxKey).(*config.BridgeConfig)

	p.JSON(w, r, http.StatusOK, cfg)
}

func (p *Presenter) GetBridgeInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg, _ := ctx.Value(BridgeCfgCtxKey).(*config.BridgeConfig)

	homeInfo, err := p.getBridgeSideInfo(ctx, cfg.ID, cfg.Home)
	if err != nil {
		p.Error(w, r, fmt.Errorf("failed to get home bridge info: %w", err))
		return
	}
	foreignInfo, err := p.getBridgeSideInfo(ctx, cfg.ID, cfg.Foreign)
	if err != nil {
		p.Error(w, r, fmt.Errorf("failed to get foreign bridge info: %w", err))
		return
	}

	p.JSON(w, r, http.StatusOK, &BridgeInfo{
		BridgeID: cfg.ID,
		Mode:     cfg.BridgeMode,
		Home:     homeInfo,
		Foreign:  foreignInfo,
	})
}

func (p *Presenter) GetBridgeValidators(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg, _ := ctx.Value(BridgeCfgCtxKey).(*config.BridgeConfig)

	homeValidators, err := p.repo.BridgeValidators.FindActiveValidators(ctx, cfg.ID, cfg.Home.Chain.ChainID)
	if err != nil {
		p.Error(w, r, fmt.Errorf("failed to find home validators: %w", err))
		return
	}

	foreignValidators, err := p.repo.BridgeValidators.FindActiveValidators(ctx, cfg.ID, cfg.Home.Chain.ChainID)
	if err != nil {
		p.Error(w, r, fmt.Errorf("failed to find home validators: %w", err))
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
				p.Error(w, r, fmt.Errorf("failed to find latest validator confirmation: %w", err))
				return
			}
		} else {
			valInfo.LastConfirmation, err = p.getTxInfo(ctx, confirmation.LogID)
			if err != nil {
				p.Error(w, r, fmt.Errorf("failed to get tx info: %w", err))
				return
			}
		}
		res.Validators = append(res.Validators, valInfo)
	}

	p.JSON(w, r, http.StatusOK, res)
}

func (p *Presenter) getFilteredLogs(ctx context.Context) ([]*entity.Log, error) {
	filter, _ := ctx.Value(FilterCtxKey).(*FilterContext)

	if filter.TxHash != nil {
		logs, err := p.repo.Logs.FindByTxHash(ctx, *filter.TxHash)
		if err != nil {
			return nil, err
		}
		if filter.ChainID != nil {
			newLogs := make([]*entity.Log, 0, len(logs))
			for _, log := range logs {
				if log.ChainID == *filter.ChainID {
					newLogs = append(newLogs, log)
				}
			}
			logs = newLogs
		}
		return logs, nil
	}

	if filter.ChainID == nil {
		return nil, errors.New("chainId query parameter is missing")
	}
	if filter.FromBlock == nil || filter.ToBlock == nil {
		return nil, errors.New("block query parameters are missing")
	}
	return p.repo.Logs.FindByBlockRange(ctx, *filter.ChainID, nil, *filter.FromBlock, *filter.ToBlock)
}

func (p *Presenter) GetMessages(w http.ResponseWriter, r *http.Request) {
	logs, err := p.getFilteredLogs(r.Context())
	if err != nil {
		p.Error(w, r, fmt.Errorf("can't filter logs: %w", err))
		return
	}

	res := p.searchForMessagesInLogs(r.Context(), logs)
	p.JSON(w, r, http.StatusOK, res)
}

func (p *Presenter) GetLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := p.getFilteredLogs(r.Context())
	if err != nil {
		p.Error(w, r, fmt.Errorf("can't filter logs: %w", err))
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

	p.JSON(w, r, http.StatusOK, res)
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

	var messageInfo interface{}
	var events []*EventInfo
	msg, err := p.repo.Messages.FindByMsgHash(ctx, sent.BridgeID, sent.MsgHash)
	if err != nil && errors.Is(err, db.ErrNotFound) {
		ercToNativeMsg, err2 := p.repo.ErcToNativeMessages.FindByMsgHash(ctx, sent.BridgeID, sent.MsgHash)
		if err2 != nil {
			return nil, err2
		}
		messageInfo = ercToNativeMessageToInfo(ercToNativeMsg)
		events, err = p.buildMessageEvents(ctx, ercToNativeMsg.BridgeID, ercToNativeMsg.MsgHash, ercToNativeMsg.MsgHash)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		messageInfo = messageToInfo(msg)
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

func (p *Presenter) searchSignedMessage(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	sig, err := p.repo.SignedMessages.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	var messageInfo interface{}
	var events []*EventInfo
	msg, err := p.repo.Messages.FindByMsgHash(ctx, sig.BridgeID, sig.MsgHash)
	if err != nil && errors.Is(err, db.ErrNotFound) {
		ercToNativeMsg, err2 := p.repo.ErcToNativeMessages.FindByMsgHash(ctx, sig.BridgeID, sig.MsgHash)
		if err2 != nil {
			return nil, err2
		}
		messageInfo = ercToNativeMessageToInfo(ercToNativeMsg)
		events, err = p.buildMessageEvents(ctx, ercToNativeMsg.BridgeID, ercToNativeMsg.MsgHash, ercToNativeMsg.MsgHash)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		messageInfo = messageToInfo(msg)
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

func (p *Presenter) searchExecutedMessage(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	executed, err := p.repo.ExecutedMessages.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	var messageInfo interface{}
	var events []*EventInfo
	msg, err := p.repo.Messages.FindByMessageID(ctx, executed.BridgeID, executed.MessageID)
	if err != nil && errors.Is(err, db.ErrNotFound) {
		ercToNativeMsg, err2 := p.repo.ErcToNativeMessages.FindByMsgHash(ctx, executed.BridgeID, executed.MessageID)
		if err2 != nil {
			return nil, err2
		}
		messageInfo = ercToNativeMessageToInfo(ercToNativeMsg)
		events, err = p.buildMessageEvents(ctx, ercToNativeMsg.BridgeID, ercToNativeMsg.MsgHash, ercToNativeMsg.MsgHash)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		messageInfo = messageToInfo(msg)
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

	req, err := p.repo.InformationRequests.FindByMessageID(ctx, sent.BridgeID, sent.MessageID)
	if err != nil {
		return nil, err
	}
	events, err := p.buildInformationRequestEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Message:       informationRequestToInfo(req),
		RelatedEvents: events,
	}, nil
}

func (p *Presenter) searchSignedInformationRequest(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	signed, err := p.repo.SignedInformationRequests.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	req, err := p.repo.InformationRequests.FindByMessageID(ctx, signed.BridgeID, signed.MessageID)
	if err != nil {
		return nil, err
	}
	events, err := p.buildInformationRequestEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Message:       informationRequestToInfo(req),
		RelatedEvents: events,
	}, nil
}

func (p *Presenter) searchExecutedInformationRequest(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	executed, err := p.repo.ExecutedInformationRequests.FindByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	req, err := p.repo.InformationRequests.FindByMessageID(ctx, executed.BridgeID, executed.MessageID)
	if err != nil {
		return nil, err
	}
	events, err := p.buildInformationRequestEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Message:       informationRequestToInfo(req),
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
		Link:        logToTxLink(log),
	}, nil
}
