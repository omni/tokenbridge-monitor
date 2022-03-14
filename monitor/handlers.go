package monitor

import (
	"amb-monitor/entity"
	"amb-monitor/ethclient"
	"amb-monitor/repository"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type EventHandler func(ctx context.Context, log *entity.Log, data map[string]interface{}) error

type BridgeEventHandler struct {
	repo       *repository.Repo
	bridgeID   string
	homeClient *ethclient.Client
}

func NewBridgeEventHandler(repo *repository.Repo, bridgeID string, homeClient *ethclient.Client) *BridgeEventHandler {
	return &BridgeEventHandler{
		repo:       repo,
		bridgeID:   bridgeID,
		homeClient: homeClient,
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
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  msgHash,
		Signer:   validator,
	})
}

func (p *BridgeEventHandler) HandleSignedForAffirmation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	msgHash := data["messageHash"].([32]byte)
	validator := data["signer"].(common.Address)

	return p.repo.SignedMessages.Ensure(ctx, &entity.SignedMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  msgHash,
		Signer:   validator,
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

func (p *BridgeEventHandler) HandleUserRequestForInformation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	messageID := data["messageId"].([32]byte)
	requestSelector := data["requestSelector"].([32]byte)
	sender := data["sender"].(common.Address)
	requestData := data["data"].([]byte)

	err := p.repo.InformationRequests.Ensure(ctx, &entity.InformationRequest{
		BridgeID:        p.bridgeID,
		MessageID:       messageID,
		Direction:       entity.DirectionHomeToForeign,
		RequestSelector: requestSelector,
		Sender:          sender,
		Executor:        sender,
		Data:            requestData,
	})
	if err != nil {
		return err
	}
	return p.repo.SentInformationRequests.Ensure(ctx, &entity.SentInformationRequest{
		LogID:     log.ID,
		BridgeID:  p.bridgeID,
		MessageID: messageID,
	})
}

func (p *BridgeEventHandler) HandleSignedForInformation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	messageID := data["messageId"].([32]byte)
	validator := data["signer"].(common.Address)

	tx, err := p.homeClient.TransactionByHash(ctx, log.TransactionHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction by hash %s: %w", log.TransactionHash, err)
	}

	return p.repo.SignedInformationRequests.Ensure(ctx, &entity.SignedInformationRequest{
		LogID:     log.ID,
		BridgeID:  p.bridgeID,
		MessageID: messageID,
		Data:      unmarshalConfirmInformationResult(tx.Data()),
		Signer:    validator,
	})
}

func (p *BridgeEventHandler) HandleInformationRetrieved(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	messageID := data["messageId"].([32]byte)
	status := data["status"].(bool)
	callbackStatus := data["callbackStatus"].(bool)

	tx, err := p.homeClient.TransactionByHash(ctx, log.TransactionHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction by hash %s: %w", log.TransactionHash, err)
	}

	return p.repo.ExecutedInformationRequests.Ensure(ctx, &entity.ExecutedInformationRequest{
		LogID:          log.ID,
		BridgeID:       p.bridgeID,
		MessageID:      messageID,
		Status:         status,
		CallbackStatus: callbackStatus,
		Data:           unmarshalConfirmInformationResult(tx.Data()),
	})
}

func (p *BridgeEventHandler) HandleValidatorAdded(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	validator := data["validator"].(common.Address)
	return p.repo.BridgeValidators.Ensure(ctx, &entity.BridgeValidator{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		ChainID:  log.ChainID,
		Address:  validator,
	})
}

func (p *BridgeEventHandler) HandleValidatorRemoved(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	validator := data["validator"].(common.Address)
	val, err := p.repo.BridgeValidators.FindActiveValidator(ctx, p.bridgeID, log.ChainID, validator)
	if err != nil {
		return err
	}
	if val == nil {
		return nil
	}
	val.RemovedLogID = &log.ID
	return p.repo.BridgeValidators.Ensure(ctx, val)
}
