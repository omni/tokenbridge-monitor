package monitor

import (
	"amb-monitor/entity"
	"amb-monitor/repository"
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type EventHandler func(ctx context.Context, log *entity.Log, data map[string]interface{}) error

type BridgeEventHandler struct {
	repo     *repository.Repo
	bridgeID string
}

func NewBridgeEventHandler(repo *repository.Repo, bridgeID string) *BridgeEventHandler {
	return &BridgeEventHandler{
		repo:     repo,
		bridgeID: bridgeID,
	}
}

func (p *BridgeEventHandler) HandleUserRequestForAffirmation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	encodedData := data["encodedData"].([]byte)
	message := unmarshalMessage(p.bridgeID, entity.DirectionForeignToHome, encodedData)
	err := p.repo.Messages.Ensure(ctx, message)
	if err != nil {
		return err
	}
	return p.repo.SentMessages.Ensure(ctx, &entity.SentMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  message.MsgHash,
	})
}

func (p *BridgeEventHandler) HandleLegacyUserRequestForAffirmation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	encodedData := data["encodedData"].([]byte)
	encodedData = append(log.TransactionHash[:], encodedData...)
	message := unmarshalLegacyMessage(p.bridgeID, entity.DirectionForeignToHome, encodedData)
	err := p.repo.Messages.Ensure(ctx, message)
	if err != nil {
		return err
	}
	return p.repo.SentMessages.Ensure(ctx, &entity.SentMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  message.MsgHash,
	})
}

func (p *BridgeEventHandler) HandleUserRequestForSignature(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	encodedData := data["encodedData"].([]byte)
	message := unmarshalMessage(p.bridgeID, entity.DirectionHomeToForeign, encodedData)
	err := p.repo.Messages.Ensure(ctx, message)
	if err != nil {
		return err
	}
	return p.repo.SentMessages.Ensure(ctx, &entity.SentMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  message.MsgHash,
	})
}

func (p *BridgeEventHandler) HandleLegacyUserRequestForSignature(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	encodedData := data["encodedData"].([]byte)
	encodedData = append(log.TransactionHash[:], encodedData...)
	message := unmarshalLegacyMessage(p.bridgeID, entity.DirectionHomeToForeign, encodedData)
	err := p.repo.Messages.Ensure(ctx, message)
	if err != nil {
		return err
	}
	return p.repo.SentMessages.Ensure(ctx, &entity.SentMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  message.MsgHash,
	})
}

func (p *BridgeEventHandler) HandleSignedForUserRequest(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	msgHash := data["messageHash"].([32]byte)
	validator := data["signer"].(common.Address)

	return p.repo.SignedMessages.Ensure(ctx, &entity.SignedMessage{
		LogID:         log.ID,
		BridgeID:      p.bridgeID,
		MsgHash:       msgHash,
		Signer:        validator,
	})
}

func (p *BridgeEventHandler) HandleSignedForAffirmation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	msgHash := data["messageHash"].([32]byte)
	validator := data["signer"].(common.Address)

	return p.repo.SignedMessages.Ensure(ctx, &entity.SignedMessage{
		LogID:         log.ID,
		BridgeID:      p.bridgeID,
		MsgHash:       msgHash,
		Signer:        validator,
	})
}

func (p *BridgeEventHandler) HandleRelayedMessage(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	messageID := data["messageId"].([32]byte)
	status := data["status"].(bool)

	return p.repo.ExecutedMessages.Ensure(ctx, &entity.ExecutedMessage{
		LogID:     log.ID,
		BridgeID:  p.bridgeID,
		MessageID: messageID,
		Status:    status,
	})
}

func (p *BridgeEventHandler) HandleAffirmationCompleted(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	messageID := data["messageId"].([32]byte)
	status := data["status"].(bool)

	return p.repo.ExecutedMessages.Ensure(ctx, &entity.ExecutedMessage{
		LogID:     log.ID,
		BridgeID:  p.bridgeID,
		MessageID: messageID,
		Status:    status,
	})
}

func (p *BridgeEventHandler) HandleCollectedSignatures(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	msgHash := data["messageHash"].([32]byte)
	relayer := data["authorityResponsibleForRelay"].(common.Address)
	numSignatures := data["NumberOfCollectedSignatures"].(*big.Int)

	return p.repo.CollectedMessages.Ensure(ctx, &entity.CollectedMessage{
		LogID:             log.ID,
		BridgeID:          p.bridgeID,
		MsgHash:           msgHash,
		ResponsibleSigner: relayer,
		NumSignatures:     uint(numSignatures.Uint64()),
	})
}
