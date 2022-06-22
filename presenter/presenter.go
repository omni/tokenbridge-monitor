package presenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/contract"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
	"github.com/poanetwork/tokenbridge-monitor/logging"
	"github.com/poanetwork/tokenbridge-monitor/presenter/http/middleware"
	"github.com/poanetwork/tokenbridge-monitor/presenter/http/render"
	"github.com/poanetwork/tokenbridge-monitor/repository"
	"github.com/poanetwork/tokenbridge-monitor/utils"
)

var (
	ErrMissingChainID             = errors.New("chainId query parameter is missing")
	ErrMissingBlockQueryParams    = errors.New("block query parameters are missing")
	ErrMissingMsgHashAndMessageID = errors.New("msgHash and messageID can't be both nil")
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
		r.Get("/pending", p.GetPendingMessages)
		r.Post("/unsigned", p.GetMessagesWithMissingSignatures)
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
	p.root.Group(func(r chi.Router) {
		r.Use(middleware.GetChainConfigMiddleware(p.cfg))
		r.Use(middleware.GetBlockNumberMiddleware)
		r.Use(middleware.GetTxHashMiddleware)
		r.Use(middleware.GetFilterMiddleware)
		r.Get("/logs", p.GetLogs)
		r.Get("/messages", p.GetMessages)
	})
	return http.ListenAndServe(addr, p.root)
}

func (p *Presenter) findActiveValidatorAddresses(ctx context.Context, bridgeID, chainID string) ([]common.Address, error) {
	validators, err := p.repo.BridgeValidators.FindActiveValidators(ctx, bridgeID, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to find validators for bridge id: %w", err)
	}
	validatorAddresses := make([]common.Address, len(validators))
	for i, v := range validators {
		validatorAddresses[i] = v.Address
	}
	return validatorAddresses, nil
}

func (p *Presenter) getBridgeSideInfo(ctx context.Context, bridgeID string, cfg *config.BridgeSideConfig) (*BridgeSideInfo, error) {
	cursor, err := p.repo.LogsCursors.GetByChainIDAndAddress(ctx, cfg.Chain.ChainID, cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get home bridge cursor: %w", err)
	}

	lastFetchedBlockTime, err := p.getBlockTimeOrDefault(ctx, cfg.Chain.ChainID, cursor.LastFetchedBlock)
	if err != nil {
		return nil, err
	}
	lastProcessedBlockTime := lastFetchedBlockTime

	if cursor.LastFetchedBlock != cursor.LastProcessedBlock {
		lastProcessedBlockTime, err = p.getBlockTimeOrDefault(ctx, cfg.Chain.ChainID, cursor.LastProcessedBlock)
		if err != nil {
			return nil, err
		}
	}

	validators, err := p.findActiveValidatorAddresses(ctx, bridgeID, cfg.Chain.ChainID)
	if err != nil {
		return nil, err
	}

	return &BridgeSideInfo{
		Chain:                  cfg.ChainName,
		ChainID:                cfg.Chain.ChainID,
		BridgeAddress:          cfg.Address,
		LastFetchedBlock:       cursor.LastFetchedBlock,
		LastFetchBlockTime:     lastFetchedBlockTime,
		LastProcessedBlock:     cursor.LastProcessedBlock,
		LastProcessedBlockTime: lastProcessedBlockTime,
		Validators:             validators,
	}, nil
}

func (p *Presenter) getBlockTimeOrDefault(ctx context.Context, chainID string, blockNumber uint) (time.Time, error) {
	bt, err := p.repo.BlockTimestamps.GetByBlockNumber(ctx, chainID, blockNumber)
	if err != nil {
		return time.Time{}, db.IgnoreErrNotFound(fmt.Errorf("failed to get block timestamp: %w", err))
	}
	return bt.Timestamp, nil
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

	homeValidators, err := p.findActiveValidatorAddresses(ctx, cfg.ID, cfg.Home.Chain.ChainID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("failed to find home validators: %w", err))
		return
	}

	foreignValidators, err := p.findActiveValidatorAddresses(ctx, cfg.ID, cfg.Home.Chain.ChainID)
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
		if seenValidators[val] {
			continue
		}
		seenValidators[val] = true

		valInfo := &ValidatorInfo{
			Address: val,
		}
		confirmation, err2 := p.repo.SignedMessages.GetLatest(ctx, cfg.ID, cfg.Home.Chain.ChainID, val)
		if err2 != nil {
			if !errors.Is(err2, db.ErrNotFound) {
				render.Error(w, r, fmt.Errorf("failed to find latest validator confirmation: %w", err2))
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

func (p *Presenter) GetPendingMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := middleware.BridgeConfig(ctx)

	msgs, err := p.repo.FindPendingMessages(ctx, cfg.ID, cfg.BridgeMode)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't find pending messages: %w", err))
		return
	}
	res := make([]interface{}, len(msgs))
	for i, m := range msgs {
		res[i] = NewBridgeMessageInfo(m)
	}
	render.JSON(w, r, http.StatusOK, res)
}

//nolint:funlen,cyclop
func (p *Presenter) GetMessagesWithMissingSignatures(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := middleware.BridgeConfig(ctx)

	foreignClient, err := ethclient.NewClient(cfg.Foreign.Chain.RPC.Host, cfg.Foreign.Chain.RPC.Timeout, cfg.Foreign.Chain.ChainID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't connect to foreign chain: %w", err))
		return
	}
	defer foreignClient.Close()

	bridgeContract := contract.NewBridgeContract(foreignClient, cfg.Foreign.Address, cfg.BridgeMode)
	requiredSignatures, err := bridgeContract.RequiredSignatures(ctx)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't get required signatures: %w", err))
		return
	}

	validators, err := p.findActiveValidatorAddresses(ctx, cfg.ID, cfg.Foreign.Chain.ChainID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't get active validators: %w", err))
		return
	}

	msgs, err := p.repo.FindPendingMessages(ctx, cfg.ID, cfg.BridgeMode)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't find pending messages: %w", err))
		return
	}
	msgHashes := make([]common.Hash, len(msgs))
	messages := make(map[common.Hash]entity.BridgeMessage, len(msgs))
	rawMessages := make(map[common.Hash][]byte, len(msgs))
	sentTxLinks := make(map[common.Hash]string, len(msgs))
	for i, msg := range msgs {
		if msg.GetDirection() == entity.DirectionHomeToForeign {
			msgHashes[i] = msg.GetMsgHash()
			messages[msg.GetMsgHash()] = msg
			rawMessages[msg.GetMsgHash()] = msg.GetRawMessage()
		}
	}

	signatures, err := p.repo.SignedMessages.FindByMsgHashes(ctx, cfg.ID, msgHashes)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't find signed signatures: %w", err))
		return
	}

	sentMsgs, err := p.repo.SentMessages.FindByMsgHashes(ctx, cfg.ID, msgHashes)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't find sent messages: %w", err))
		return
	}

	logIDs := make([]uint, len(sentMsgs))
	sentMsgHashMap := make(map[uint]common.Hash, len(sentMsgs))
	for i, sentMsg := range sentMsgs {
		logIDs[i] = sentMsg.LogID
		sentMsgHashMap[sentMsg.LogID] = sentMsg.MsgHash
	}

	logs, err := p.repo.Logs.FindByIDs(ctx, logIDs)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't find logs: %w", err))
		return
	}
	for _, log := range logs {
		sentTxLinks[sentMsgHashMap[log.ID]] = p.cfg.GetChainConfig(log.ChainID).FormatTxLink(log.TransactionHash)
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	manualSigners, err := p.parseManualSignatures(r, rawMessages)
	if err != nil {
		render.Error(w, r, fmt.Errorf("can't parse manual signatures: %w", err))
		return
	}

	signersMap := p.makeSignersMap(msgHashes, signatures, manualSigners)

	res := make([]*UnsignedMessageInfo, 0, len(messages))
	for hash, msg := range messages {
		var signers, missingSigners []common.Address
		for _, signer := range validators {
			if signersMap[hash][signer] {
				signers = append(signers, signer)
			} else {
				missingSigners = append(missingSigners, signer)
			}
		}
		if uint(len(signers)) < requiredSignatures {
			res = append(res, &UnsignedMessageInfo{
				Message:        NewBridgeMessageInfo(msg),
				Link:           sentTxLinks[hash],
				Signers:        signers,
				MissingSigners: missingSigners,
			})
		}
	}
	render.JSON(w, r, http.StatusOK, UnsignedMessagesInfo{
		RequiredSignatures:    requiredSignatures,
		ActiveValidators:      validators,
		TotalPendingMessages:  uint(len(msgHashes)),
		TotalUnsignedMessages: uint(len(res)),
		UnsignedMessages:      res,
	})
}

func (p *Presenter) makeSignersMap(msgHashes []common.Hash, signatures []*entity.SignedMessage, manualSigners map[common.Hash][]common.Address) map[common.Hash]map[common.Address]bool {
	signersMap := make(map[common.Hash]map[common.Address]bool, len(msgHashes))
	for _, hash := range msgHashes {
		signersMap[hash] = make(map[common.Address]bool, 10)
	}
	for _, sig := range signatures {
		signersMap[sig.MsgHash][sig.Signer] = true
	}
	for hash, signers := range manualSigners {
		if signersMap[hash] == nil {
			continue
		}
		for _, signer := range signers {
			signersMap[hash][signer] = true
		}
	}
	return signersMap
}

func (p *Presenter) parseManualSignatures(r *http.Request, messages map[common.Hash][]byte) (map[common.Hash][]common.Address, error) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		return nil, fmt.Errorf("can't parse multipart form: %w", err)
	}
	res := make(map[common.Hash][]common.Address, 100)
	for _, hdr := range r.MultipartForm.File["signatures"] {
		file, err2 := hdr.Open()
		if err2 != nil {
			return nil, fmt.Errorf("can't open file: %w", err2)
		}
		signatures := make(map[common.Hash]hexutil.Bytes)
		err = json.NewDecoder(file).Decode(&signatures)
		if err != nil {
			return nil, fmt.Errorf("can't decode json file: %w", err)
		}
		for hash, sig := range signatures {
			signer, err3 := utils.RestoreSignerAddress(messages[hash], sig)
			if err3 != nil {
				return nil, err3
			}
			res[hash] = append(res[hash], signer)
		}
	}
	return res, nil
}

func (p *Presenter) getFilteredLogs(ctx context.Context) ([]*entity.Log, error) {
	filter := middleware.GetFilterContext(ctx)

	if filter.TxHash == nil {
		if filter.ChainID == nil {
			return nil, ErrMissingChainID
		}
		if filter.FromBlock == nil || filter.ToBlock == nil {
			return nil, ErrMissingBlockQueryParams
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

	res := make([]*LogInfo, len(logs))
	for i, log := range logs {
		res[i] = NewLogInfo(log)
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
	sent, err := p.repo.SentMessages.GetByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForMessage(ctx, sent.BridgeID, &sent.MsgHash, nil)
}

func (p *Presenter) searchSignedMessage(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	sig, err := p.repo.SignedMessages.GetByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForMessage(ctx, sig.BridgeID, &sig.MsgHash, nil)
}

func (p *Presenter) searchExecutedMessage(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	executed, err := p.repo.ExecutedMessages.GetByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForMessage(ctx, executed.BridgeID, nil, &executed.MessageID)
}

func (p *Presenter) buildSearchResultForMessage(ctx context.Context, bridgeID string, msgHash, messageID *common.Hash) (*SearchResult, error) {
	if msgHash == nil && messageID == nil {
		return nil, ErrMissingMsgHashAndMessageID
	}
	var msg entity.BridgeMessage
	var err error
	var searchID common.Hash
	if msgHash != nil {
		searchID = *msgHash
		msg, err = p.repo.Messages.GetByMsgHash(ctx, bridgeID, *msgHash)
	} else {
		searchID = *messageID
		msg, err = p.repo.Messages.GetByMessageID(ctx, bridgeID, *messageID)
	}
	if errors.Is(err, db.ErrNotFound) {
		msg, err = p.repo.ErcToNativeMessages.GetByMsgHash(ctx, bridgeID, searchID)
	}
	if err != nil {
		return nil, err
	}
	events, err := p.buildMessageEvents(ctx, bridgeID, msg.GetMsgHash(), msg.GetMessageID())
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Message:       NewBridgeMessageInfo(msg),
		RelatedEvents: events,
	}, nil
}

func (p *Presenter) searchSentInformationRequest(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	sent, err := p.repo.SentInformationRequests.GetByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForInformationRequest(ctx, sent.BridgeID, sent.MessageID)
}

func (p *Presenter) searchSignedInformationRequest(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	signed, err := p.repo.SignedInformationRequests.GetByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForInformationRequest(ctx, signed.BridgeID, signed.MessageID)
}

func (p *Presenter) searchExecutedInformationRequest(ctx context.Context, log *entity.Log) (*SearchResult, error) {
	executed, err := p.repo.ExecutedInformationRequests.GetByLogID(ctx, log.ID)
	if err != nil {
		return nil, err
	}

	return p.buildSearchResultForInformationRequest(ctx, executed.BridgeID, executed.MessageID)
}

func (p *Presenter) buildSearchResultForInformationRequest(ctx context.Context, bridgeID string, messageID common.Hash) (*SearchResult, error) {
	req, err := p.repo.InformationRequests.GetByMessageID(ctx, bridgeID, messageID)
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
	sent, err := p.repo.SentMessages.GetByMsgHash(ctx, bridgeID, msgHash)
	if err = db.IgnoreErrNotFound(err); err != nil {
		return nil, err
	}
	signed, err := p.repo.SignedMessages.FindByMsgHashes(ctx, bridgeID, []common.Hash{msgHash})
	if err != nil {
		return nil, err
	}
	collected, err := p.repo.CollectedMessages.GetByMsgHash(ctx, bridgeID, msgHash)
	if err = db.IgnoreErrNotFound(err); err != nil {
		return nil, err
	}
	executed, err := p.repo.ExecutedMessages.GetByMessageID(ctx, bridgeID, messageID)
	if err = db.IgnoreErrNotFound(err); err != nil {
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
	sent, err := p.repo.SentInformationRequests.GetByMessageID(ctx, req.BridgeID, req.MessageID)
	if err = db.IgnoreErrNotFound(err); err != nil {
		return nil, err
	}
	signed, err := p.repo.SignedInformationRequests.FindByMessageID(ctx, req.BridgeID, req.MessageID)
	if err != nil {
		return nil, err
	}
	executed, err := p.repo.ExecutedInformationRequests.GetByMessageID(ctx, req.BridgeID, req.MessageID)
	if err = db.IgnoreErrNotFound(err); err != nil {
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
		Link:        p.cfg.GetChainConfig(log.ChainID).FormatTxLink(log.TransactionHash),
	}, nil
}
