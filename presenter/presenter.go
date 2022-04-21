package presenter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"tokenbridge-monitor/config"
	"tokenbridge-monitor/entity"
	"tokenbridge-monitor/logging"
	"tokenbridge-monitor/repository"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Presenter struct {
	logger  logging.Logger
	repo    *repository.Repo
	bridges map[string]*config.BridgeConfig
	root    chi.Router
}

func NewPresenter(logger logging.Logger, repo *repository.Repo, bridges map[string]*config.BridgeConfig) *Presenter {
	return &Presenter{
		logger:  logger,
		repo:    repo,
		bridges: bridges,
		root:    chi.NewMux(),
	}
}

func (p *Presenter) Serve(addr string) error {
	p.logger.WithField("addr", addr).Info("starting presenter service")
	p.root.Use(middleware.Throttle(5))
	p.root.Use(middleware.RequestID)
	p.root.Use(NewRequestLogger(p.logger))
	p.root.Get("/tx/{txHash:0x[0-9a-fA-F]{64}}", p.wrapJSONHandler(p.SearchTx))
	p.root.Get("/block/{chainID:[0-9]+}/{blockNumber:[0-9]+}", p.wrapJSONHandler(p.SearchBlock))
	p.root.Get("/bridge/{bridgeID:[0-9a-zA-Z_\\-]+}/validators", p.wrapJSONHandler(p.SearchValidators))
	return http.ListenAndServe(addr, p.root)
}

func (p *Presenter) wrapJSONHandler(handler func(ctx context.Context) (interface{}, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		res, err := handler(ctx)
		if err != nil {
			p.logger.WithError(err).Error("failed to handle request")
			w.WriteHeader(http.StatusInternalServerError)
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err = enc.Encode(res); err != nil {
			p.logger.WithError(err).Error("failed to marshal JSON result")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func (p *Presenter) SearchTx(ctx context.Context) (interface{}, error) {
	txHash := common.HexToHash(chi.URLParamFromCtx(ctx, "txHash"))

	logs, err := p.repo.Logs.FindByTxHash(ctx, txHash)
	if err != nil {
		p.logger.WithError(err).Error("failed to find logs by tx hash")
		return nil, err
	}
	return p.searchInLogs(ctx, logs), nil
}

func (p *Presenter) SearchBlock(ctx context.Context) (interface{}, error) {
	chainID := chi.URLParamFromCtx(ctx, "chainID")
	blockNumber, err := strconv.ParseUint(chi.URLParamFromCtx(ctx, "blockNumber"), 10, 64)
	if err != nil {
		p.logger.WithError(err).Error("failed to parse blockNumber")
		return nil, err
	}

	logs, err := p.repo.Logs.FindByBlockNumber(ctx, chainID, uint(blockNumber))
	if err != nil {
		p.logger.WithError(err).Error("failed to find logs by block number")
		return nil, err
	}

	return p.searchInLogs(ctx, logs), nil
}

func (p *Presenter) SearchValidators(ctx context.Context) (interface{}, error) {
	bridgeID := chi.URLParamFromCtx(ctx, "bridgeID")

	if p.bridges[bridgeID] == nil {
		return nil, fmt.Errorf("bridge %q not found", bridgeID)
	}

	cfg := p.bridges[bridgeID]
	res := ValidatorsResult{
		BridgeID: bridgeID,
		Home: &ValidatorSideResult{
			ChainID: cfg.Home.Chain.ChainID,
		},
		Foreign: &ValidatorSideResult{
			ChainID: cfg.Foreign.Chain.ChainID,
		},
	}

	homeCursor, err := p.repo.LogsCursors.GetByChainIDAndAddress(ctx, res.Home.ChainID, cfg.Home.Address)
	if err != nil {
		p.logger.WithError(err).Error("failed to get home bridge cursor")
		return nil, err
	}
	foreignCursor, err := p.repo.LogsCursors.GetByChainIDAndAddress(ctx, res.Foreign.ChainID, cfg.Foreign.Address)
	if err != nil {
		p.logger.WithError(err).Error("failed to get foreign bridge cursor")
		return nil, err
	}

	res.Home.BlockNumber = homeCursor.LastProcessedBlock
	res.Foreign.BlockNumber = foreignCursor.LastProcessedBlock

	validators, err := p.repo.BridgeValidators.FindActiveValidators(ctx, bridgeID)
	if err != nil {
		p.logger.WithError(err).Error("failed to find validators for bridge id")
		return nil, err
	}

	seenValidators := make(map[common.Address]bool, len(validators))
	for _, val := range validators {
		if seenValidators[val.Address] {
			continue
		}
		seenValidators[val.Address] = true

		confirmation, err := p.repo.SignedMessages.FindLatest(ctx, bridgeID, res.Home.ChainID, val.Address)
		if err != nil {
			p.logger.WithError(err).Error("failed to find latest validator confirmation")
			return nil, err
		}
		valInfo := &ValidatorInfo{
			Signer: val.Address,
		}
		if confirmation != nil {
			valInfo.LastConfirmation, err = p.getTxInfo(ctx, confirmation.LogID)
			if err != nil {
				p.logger.WithError(err).Error("failed to get tx info")
				return nil, err
			}
		}
		res.Validators = append(res.Validators, valInfo)
	}

	return res, nil
}

func (p *Presenter) searchInLogs(ctx context.Context, logs []*entity.Log) []*SearchResult {
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
			if res, err := task(ctx, log); err != nil {
				p.logger.WithError(err).Error("failed to execute search task")
			} else if res != nil {
				for _, e := range res.RelatedEvents {
					if e.LogID == log.ID {
						res.Event = e
						break
					}
				}
				if res.Event == nil {
					p.logger.WithError(err).Error("tx event not found in related events")
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
	if sent == nil {
		return nil, nil
	}

	var messageInfo interface{}
	var events []*EventInfo
	msg, err := p.repo.Messages.FindByMsgHash(ctx, sent.BridgeID, sent.MsgHash)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		ercToNativeMsg, err2 := p.repo.ErcToNativeMessages.FindByMsgHash(ctx, sent.BridgeID, sent.MsgHash)
		if err2 != nil {
			return nil, err2
		}
		if ercToNativeMsg == nil {
			return nil, nil
		}
		messageInfo = ercToNativeMessageToInfo(ercToNativeMsg)
		events, err = p.buildMessageEvents(ctx, ercToNativeMsg.BridgeID, ercToNativeMsg.MsgHash, ercToNativeMsg.MsgHash)
	} else {
		messageInfo = messageToInfo(msg)
		events, err = p.buildMessageEvents(ctx, msg.BridgeID, msg.MsgHash, msg.MessageID)
	}
	if err != nil {
		return nil, err
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
	if sig == nil {
		return nil, nil
	}

	var messageInfo interface{}
	var events []*EventInfo
	msg, err := p.repo.Messages.FindByMsgHash(ctx, sig.BridgeID, sig.MsgHash)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		ercToNativeMsg, err2 := p.repo.ErcToNativeMessages.FindByMsgHash(ctx, sig.BridgeID, sig.MsgHash)
		if err2 != nil {
			return nil, err2
		}
		if ercToNativeMsg == nil {
			return nil, nil
		}
		messageInfo = ercToNativeMessageToInfo(ercToNativeMsg)
		events, err = p.buildMessageEvents(ctx, ercToNativeMsg.BridgeID, ercToNativeMsg.MsgHash, ercToNativeMsg.MsgHash)
	} else {
		messageInfo = messageToInfo(msg)
		events, err = p.buildMessageEvents(ctx, msg.BridgeID, msg.MsgHash, msg.MessageID)
	}
	if err != nil {
		return nil, err
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
	if executed == nil {
		return nil, nil
	}

	var messageInfo interface{}
	var events []*EventInfo
	msg, err := p.repo.Messages.FindByMessageID(ctx, executed.BridgeID, executed.MessageID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		ercToNativeMsg, err2 := p.repo.ErcToNativeMessages.FindByMsgHash(ctx, executed.BridgeID, executed.MessageID)
		if err2 != nil {
			return nil, err2
		}
		if ercToNativeMsg == nil {
			return nil, nil
		}
		messageInfo = ercToNativeMessageToInfo(ercToNativeMsg)
		events, err = p.buildMessageEvents(ctx, ercToNativeMsg.BridgeID, ercToNativeMsg.MsgHash, ercToNativeMsg.MsgHash)
	} else {
		messageInfo = messageToInfo(msg)
		events, err = p.buildMessageEvents(ctx, msg.BridgeID, msg.MsgHash, msg.MessageID)
	}
	if err != nil {
		return nil, err
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
	if sent == nil {
		return nil, nil
	}

	req, err := p.repo.InformationRequests.FindByMessageID(ctx, sent.BridgeID, sent.MessageID)
	if err != nil || req == nil {
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
	if signed == nil {
		return nil, nil
	}

	req, err := p.repo.InformationRequests.FindByMessageID(ctx, signed.BridgeID, signed.MessageID)
	if err != nil || req == nil {
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
	if executed == nil {
		return nil, nil
	}

	req, err := p.repo.InformationRequests.FindByMessageID(ctx, executed.BridgeID, executed.MessageID)
	if err != nil || req == nil {
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
	if err != nil {
		return nil, err
	}
	signed, err := p.repo.SignedMessages.FindByMsgHash(ctx, bridgeID, msgHash)
	if err != nil {
		return nil, err
	}
	collected, err := p.repo.CollectedMessages.FindByMsgHash(ctx, bridgeID, msgHash)
	if err != nil {
		return nil, err
	}
	executed, err := p.repo.ExecutedMessages.FindByMessageID(ctx, bridgeID, messageID)
	if err != nil {
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
	if err != nil {
		return nil, err
	}
	signed, err := p.repo.SignedInformationRequests.FindByMessageID(ctx, req.BridgeID, req.MessageID)
	if err != nil {
		return nil, err
	}
	executed, err := p.repo.ExecutedInformationRequests.FindByMessageID(ctx, req.BridgeID, req.MessageID)
	if err != nil {
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
	ts, err := p.repo.BlockTimestamps.GetByBlockNumber(ctx, log.ChainID, log.BlockNumber)
	if err != nil {
		return nil, err
	}
	return &TxInfo{
		BlockNumber: log.BlockNumber,
		Timestamp:   ts.Timestamp,
		Link:        logToTxLink(log),
	}, nil
}
