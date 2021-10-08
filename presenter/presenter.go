package presenter

import (
	"amb-monitor/entity"
	"amb-monitor/logging"
	"amb-monitor/repository"
	"context"
	"encoding/json"
	"net/http"

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
	p.root.Get("/tx/{txHash:0x[0-9a-f]{64}}", p.SearchTx)
	return http.ListenAndServe(addr, p.root)
}

func (p *Presenter) SearchTx(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txHash := common.HexToHash(chi.URLParam(r, "txHash"))

	results := make([]*SearchResult, 0, 2)
	logs, err := p.repo.Logs.FindByTxHash(ctx, txHash)
	if err != nil {
		p.logger.WithError(err).Error("failed to find logs by tx hash")
		w.WriteHeader(http.StatusInternalServerError)
	}

	for _, log := range logs {
		for _, task := range []func(context.Context, *entity.Log) (*SearchResult, error){
			p.searchSentMessage,
			p.searchSignedMessage,
			p.searchExecutedMessage,
		} {
			if res, err := task(ctx, log); err != nil {
				p.logger.WithError(err).Error("failed to execute search task")
			} else if res != nil {
				results = append(results, res)
				break
			}
		}
	}
	err = json.NewEncoder(w).Encode(results)
	if err != nil {
		p.logger.WithError(err).Error("failed marshal results")
		w.WriteHeader(http.StatusInternalServerError)
	}
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
		Message: messageToMessageInfo(msg),
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
		Message: messageToMessageInfo(msg),
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
		Message: messageToMessageInfo(msg),
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
