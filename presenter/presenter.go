package presenter

import (
	"amb-monitor/entity"
	"amb-monitor/logging"
	"amb-monitor/repository"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Presenter struct {
	logger logging.Logger
	repo   *repository.Repo
	root   chi.Router
}

func NewPresenter(logger logging.Logger, repo *repository.Repo) *Presenter {
	return &Presenter{
		logger: logger,
		repo:   repo,
		root:   chi.NewMux(),
	}
}

func (p *Presenter) Serve(addr string) error {
	p.logger.WithField("addr", addr).Info("starting presenter service")
	p.root.Use(middleware.Throttle(5))
	p.root.Use(middleware.RequestID)
	p.root.Use(NewRequestLogger(p.logger))
	p.root.Get("/tx/{txHash:0x[0-9a-fA-F]{64}}", p.SearchTx)
	p.root.Get("/block/{chainID:[0-9]+}/{blockNumber:[0-9]+}", p.SearchBlock)
	return http.ListenAndServe(addr, p.root)
}

func (p *Presenter) SearchTx(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txHash := common.HexToHash(chi.URLParam(r, "txHash"))

	logs, err := p.repo.Logs.FindByTxHash(ctx, txHash)
	if err != nil {
		p.logger.WithError(err).Error("failed to find logs by tx hash")
		w.WriteHeader(http.StatusInternalServerError)
	}

	results := p.searchInLogs(ctx, logs)
	err = json.NewEncoder(w).Encode(results)
	if err != nil {
		p.logger.WithError(err).Error("failed to marshal results")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (p *Presenter) SearchBlock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chainID := chi.URLParam(r, "chainID")
	blockNumber, err := strconv.ParseUint(chi.URLParam(r, "blockNumber"), 10, 64)
	if err != nil {
		p.logger.WithError(err).Error("failed to parse blockNumber")
		w.WriteHeader(http.StatusInternalServerError)
	}

	logs, err := p.repo.Logs.FindByBlockNumber(ctx, chainID, uint(blockNumber))
	if err != nil {
		p.logger.WithError(err).Error("failed to find logs by block number")
		w.WriteHeader(http.StatusInternalServerError)
	}

	results := p.searchInLogs(ctx, logs)
	err = json.NewEncoder(w).Encode(results)
	if err != nil {
		p.logger.WithError(err).Error("failed to marshal results")
		w.WriteHeader(http.StatusInternalServerError)
	}
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

	msg, err := p.repo.Messages.FindByMsgHash(ctx, sent.BridgeID, sent.MsgHash)
	if err != nil {
		return nil, err
	}
	events, err := p.buildMessageEvents(ctx, msg)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Bridge:  msg.BridgeID,
		Event:   "SENT_MESSAGE",
		TxHash:  log.TransactionHash,
		Message: messageToInfo(msg),
		Events:  events,
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

	msg, err := p.repo.Messages.FindByMsgHash(ctx, sig.BridgeID, sig.MsgHash)
	if err != nil {
		return nil, err
	}
	events, err := p.buildMessageEvents(ctx, msg)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Bridge:  msg.BridgeID,
		Event:   "SIGNED_MESSAGE",
		TxHash:  log.TransactionHash,
		Message: messageToInfo(msg),
		Events:  events,
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

	msg, err := p.repo.Messages.FindByMessageID(ctx, executed.BridgeID, executed.MessageID)
	if err != nil {
		return nil, err
	}
	events, err := p.buildMessageEvents(ctx, msg)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Bridge:  msg.BridgeID,
		Event:   "EXECUTED_MESSAGE",
		TxHash:  log.TransactionHash,
		Message: messageToInfo(msg),
		Events:  events,
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
	if err != nil {
		return nil, err
	}
	events, err := p.buildInformationRequestEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Bridge:  req.BridgeID,
		Event:   "SENT_INFORMATION_REQUEST",
		TxHash:  log.TransactionHash,
		Message: informationRequestToInfo(req),
		Events:  events,
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
	if err != nil {
		return nil, err
	}
	events, err := p.buildInformationRequestEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Bridge:  req.BridgeID,
		Event:   "SIGNED_INFORMATION_REQUEST",
		TxHash:  log.TransactionHash,
		Message: informationRequestToInfo(req),
		Events:  events,
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
	if err != nil {
		return nil, err
	}
	events, err := p.buildInformationRequestEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Bridge:  req.BridgeID,
		Event:   "EXECUTED_INFORMATION_REQUEST",
		TxHash:  log.TransactionHash,
		Message: informationRequestToInfo(req),
		Events:  events,
	}, nil
}

func (p *Presenter) buildMessageEvents(ctx context.Context, msg *entity.Message) ([]*EventInfo, error) {
	sent, err := p.repo.SentMessages.FindByMsgHash(ctx, msg.BridgeID, msg.MsgHash)
	if err != nil {
		return nil, err
	}
	signed, err := p.repo.SignedMessages.FindByMsgHash(ctx, msg.BridgeID, msg.MsgHash)
	if err != nil {
		return nil, err
	}
	collected, err := p.repo.CollectedMessages.FindByMsgHash(ctx, msg.BridgeID, msg.MsgHash)
	if err != nil {
		return nil, err
	}
	executed, err := p.repo.ExecutedMessages.FindByMessageID(ctx, msg.BridgeID, msg.MessageID)
	if err != nil {
		return nil, err
	}

	events := make([]*EventInfo, 0, 5)
	if sent != nil {
		events = append(events, &EventInfo{
			Event: "SENT_MESSAGE",
			LogID: sent.LogID,
		})
	}
	for _, s := range signed {
		events = append(events, &EventInfo{
			Event:  "SIGNED_MESSAGE",
			LogID:  s.LogID,
			Signer: &s.Signer,
		})
	}
	if collected != nil {
		events = append(events, &EventInfo{
			Event: "COLLECTED_SIGNATURES",
			LogID: collected.LogID,
			Count: collected.NumSignatures,
		})
	}
	if executed != nil {
		events = append(events, &EventInfo{
			Event:  "EXECUTED_MESSAGE",
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
			Event: "SENT_INFORMATION_REQUEST",
			LogID: sent.LogID,
		})
	}
	for _, s := range signed {
		events = append(events, &EventInfo{
			Event:  "SIGNED_INFORMATION_REQUEST",
			LogID:  s.LogID,
			Signer: &s.Signer,
			Data:   s.Data,
		})
	}
	if executed != nil {
		events = append(events, &EventInfo{
			Event:          "EXECUTED_INFORMATION_REQUEST",
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
