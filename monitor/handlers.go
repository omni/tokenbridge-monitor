package monitor

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/omni/tokenbridge-monitor/config"
	"github.com/omni/tokenbridge-monitor/contract/bridgeabi"
	"github.com/omni/tokenbridge-monitor/db"
	"github.com/omni/tokenbridge-monitor/entity"
	"github.com/omni/tokenbridge-monitor/ethclient"
	"github.com/omni/tokenbridge-monitor/repository"
)

type EventHandler func(ctx context.Context, log *entity.Log, data map[string]interface{}) error

type BridgeEventHandler struct {
	repo       *repository.Repo
	bridgeID   string
	homeClient ethclient.Client
	cfg        *config.BridgeConfig
}

func NewBridgeEventHandler(repo *repository.Repo, cfg *config.BridgeConfig, homeClient ethclient.Client) *BridgeEventHandler {
	return &BridgeEventHandler{
		repo:       repo,
		bridgeID:   cfg.ID,
		homeClient: homeClient,
		cfg:        cfg,
	}
}

func (p *BridgeEventHandler) HandleUserRequestForAffirmation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	encodedData, ok := data["encodedData"].([]byte)
	if !ok {
		return fmt.Errorf("encodedData type %T is invalid: %w", data["encodedData"], ErrWrongArgumentType)
	}
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
	encodedData, ok := data["encodedData"].([]byte)
	if !ok {
		return fmt.Errorf("encodedData type %T is invalid: %w", data["encodedData"], ErrWrongArgumentType)
	}
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

func (p *BridgeEventHandler) HandleErcToNativeTransfer(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	from, ok := data["from"].(common.Address)
	if !ok {
		return fmt.Errorf("from type %T is invalid: %w", data["from"], ErrWrongArgumentType)
	}
	value, ok := data["value"].(*big.Int)
	if !ok {
		return fmt.Errorf("value type %T is invalid: %w", data["value"], ErrWrongArgumentType)
	}

	for _, token := range p.cfg.Foreign.ErcToNativeTokens {
		if token.Address == log.Address {
			for _, addr := range token.BlacklistedSenders {
				if from == addr {
					return nil
				}
			}
			break
		}
	}
	filter := entity.LogsFilter{
		ChainID:   &log.ChainID,
		Addresses: []common.Address{p.cfg.Foreign.Address},
		FromBlock: &log.BlockNumber,
		ToBlock:   &log.BlockNumber,
		TxHash:    &log.TransactionHash,
		Topic0:    []common.Hash{bridgeabi.ErcToNativeUserRequestForAffirmationEventSignature},
	}
	logs, err := p.repo.Logs.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to get transaction logs for %s: %w", log.TransactionHash, err)
	}
	if len(logs) > 0 {
		return nil
	}

	valueBytes := common.BigToHash(value)
	msg := from[:]
	msg = append(msg, valueBytes[:]...)
	msg = append(msg, log.TransactionHash[:]...)
	msgHash := crypto.Keccak256Hash(msg)

	message := &entity.ErcToNativeMessage{
		BridgeID:   p.bridgeID,
		Direction:  entity.DirectionForeignToHome,
		MsgHash:    msgHash,
		Sender:     from,
		Receiver:   from,
		Value:      value.String(),
		RawMessage: msg,
	}
	err = p.repo.ErcToNativeMessages.Ensure(ctx, message)
	if err != nil {
		return err
	}
	return p.repo.SentMessages.Ensure(ctx, &entity.SentMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  msgHash,
	})
}

func (p *BridgeEventHandler) HandleErcToNativeUserRequestForAffirmation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	recipient, ok := data["recipient"].(common.Address)
	if !ok {
		return fmt.Errorf("recipient type %T is invalid: %w", data["recipient"], ErrWrongArgumentType)
	}
	value, ok := data["value"].(*big.Int)
	if !ok {
		return fmt.Errorf("value type %T is invalid: %w", data["value"], ErrWrongArgumentType)
	}

	valueBytes := common.BigToHash(value)
	msg := recipient[:]
	msg = append(msg, valueBytes[:]...)
	msg = append(msg, log.TransactionHash[:]...)
	msgHash := crypto.Keccak256Hash(msg)

	filter := entity.LogsFilter{
		ChainID:    &log.ChainID,
		Addresses:  p.cfg.Foreign.ErcToNativeTokenAddresses(log.BlockNumber, log.BlockNumber),
		FromBlock:  &log.BlockNumber,
		ToBlock:    &log.BlockNumber,
		TxHash:     &log.TransactionHash,
		Topic0:     []common.Hash{bridgeabi.ErcToNativeTransferEventSignature},
		Topic2:     []common.Hash{p.cfg.Foreign.Address.Hash()},
		DataLength: uintPtr(32),
	}
	logs, err := p.repo.Logs.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to get transaction logs for %s: %w", log.TransactionHash, err)
	}
	sender := recipient
	for _, txLog := range logs {
		transferValue := new(big.Int).SetBytes(txLog.Data)
		if value.Cmp(transferValue) == 0 {
			sender = common.BytesToAddress(txLog.Topic1[:])
		}
	}

	message := &entity.ErcToNativeMessage{
		BridgeID:   p.bridgeID,
		Direction:  entity.DirectionForeignToHome,
		MsgHash:    msgHash,
		Sender:     sender,
		Receiver:   recipient,
		Value:      value.String(),
		RawMessage: msg,
	}
	err = p.repo.ErcToNativeMessages.Ensure(ctx, message)
	if err != nil {
		return err
	}
	return p.repo.SentMessages.Ensure(ctx, &entity.SentMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  msgHash,
	})
}

func (p *BridgeEventHandler) HandleUserRequestForSignature(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	encodedData, ok := data["encodedData"].([]byte)
	if !ok {
		return fmt.Errorf("encodedData type %T is invalid: %w", data["encodedData"], ErrWrongArgumentType)
	}
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
	encodedData, ok := data["encodedData"].([]byte)
	if !ok {
		return fmt.Errorf("encodedData type %T is invalid: %w", data["encodedData"], ErrWrongArgumentType)
	}
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

func (p *BridgeEventHandler) HandleErcToNativeUserRequestForSignature(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	recipient, ok := data["recipient"].(common.Address)
	if !ok {
		return fmt.Errorf("recipient type %T is invalid: %w", data["recipient"], ErrWrongArgumentType)
	}
	value, ok := data["value"].(*big.Int)
	if !ok {
		return fmt.Errorf("value type %T is invalid: %w", data["value"], ErrWrongArgumentType)
	}

	valueBytes := common.BigToHash(value)
	msg := recipient[:]
	msg = append(msg, valueBytes[:]...)
	msg = append(msg, log.TransactionHash[:]...)
	msg = append(msg, p.cfg.Foreign.Address[:]...)
	msgHash := crypto.Keccak256Hash(msg)

	sender := recipient
	tx, err := p.homeClient.TransactionByHash(ctx, log.TransactionHash)
	if err != nil {
		return err
	}
	if tx.Value().Cmp(value) == 0 {
		sender, err = p.homeClient.TransactionSender(tx)
		if err != nil {
			return fmt.Errorf("failed to extract transaction sender: %w", err)
		}
	}

	message := &entity.ErcToNativeMessage{
		BridgeID:   p.bridgeID,
		Direction:  entity.DirectionHomeToForeign,
		MsgHash:    msgHash,
		Sender:     sender,
		Receiver:   recipient,
		Value:      value.String(),
		RawMessage: msg,
	}
	err = p.repo.ErcToNativeMessages.Ensure(ctx, message)
	if err != nil {
		return err
	}
	return p.repo.SentMessages.Ensure(ctx, &entity.SentMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  msgHash,
	})
}

func (p *BridgeEventHandler) HandleSignedForUserRequest(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	msgHash, ok := data["messageHash"].([32]byte)
	if !ok {
		return fmt.Errorf("messageHash type %T is invalid: %w", data["messageHash"], ErrWrongArgumentType)
	}
	validator, ok := data["signer"].(common.Address)
	if !ok {
		return fmt.Errorf("signer type %T is invalid: %w", data["signer"], ErrWrongArgumentType)
	}

	return p.repo.SignedMessages.Ensure(ctx, &entity.SignedMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  msgHash,
		Signer:   validator,
	})
}

func (p *BridgeEventHandler) HandleErcToNativeSignedForAffirmation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	validator, ok := data["signer"].(common.Address)
	if !ok {
		return fmt.Errorf("signer type %T is invalid: %w", data["signer"], ErrWrongArgumentType)
	}

	tx, err := p.homeClient.TransactionByHash(ctx, log.TransactionHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction by hash %s: %w", log.TransactionHash, err)
	}
	msg := tx.Data()[16:]

	return p.repo.SignedMessages.Ensure(ctx, &entity.SignedMessage{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		MsgHash:  crypto.Keccak256Hash(msg),
		Signer:   validator,
	})
}

func (p *BridgeEventHandler) HandleRelayedMessage(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	messageID, ok := data["messageId"].([32]byte)
	if !ok {
		return fmt.Errorf("messageId type %T is invalid: %w", data["messageId"], ErrWrongArgumentType)
	}
	status, ok := data["status"].(bool)
	if !ok {
		return fmt.Errorf("status type %T is invalid: %w", data["status"], ErrWrongArgumentType)
	}

	return p.repo.ExecutedMessages.Ensure(ctx, &entity.ExecutedMessage{
		LogID:     log.ID,
		BridgeID:  p.bridgeID,
		MessageID: messageID,
		Status:    status,
	})
}

func (p *BridgeEventHandler) HandleErcToNativeRelayedMessage(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	recipient, ok := data["recipient"].(common.Address)
	if !ok {
		return fmt.Errorf("recipient type %T is invalid: %w", data["recipient"], ErrWrongArgumentType)
	}
	value, ok := data["value"].(*big.Int)
	if !ok {
		return fmt.Errorf("value type %T is invalid: %w", data["value"], ErrWrongArgumentType)
	}
	transactionHash, ok := data["transactionHash"].([32]byte)
	if !ok {
		return fmt.Errorf("transactionHash type %T is invalid: %w", data["transactionHash"], ErrWrongArgumentType)
	}

	valueBytes := common.BigToHash(value)

	msg := recipient[:]
	msg = append(msg, valueBytes[:]...)
	msg = append(msg, transactionHash[:]...)
	msg = append(msg, log.Address[:]...)

	return p.repo.ExecutedMessages.Ensure(ctx, &entity.ExecutedMessage{
		LogID:     log.ID,
		BridgeID:  p.bridgeID,
		MessageID: crypto.Keccak256Hash(msg),
		Status:    true,
	})
}

func (p *BridgeEventHandler) HandleAffirmationCompleted(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	messageID, ok := data["messageId"].([32]byte)
	if !ok {
		return fmt.Errorf("messageId type %T is invalid: %w", data["messageId"], ErrWrongArgumentType)
	}
	status, ok := data["status"].(bool)
	if !ok {
		return fmt.Errorf("status type %T is invalid: %w", data["status"], ErrWrongArgumentType)
	}

	return p.repo.ExecutedMessages.Ensure(ctx, &entity.ExecutedMessage{
		LogID:     log.ID,
		BridgeID:  p.bridgeID,
		MessageID: messageID,
		Status:    status,
	})
}

func (p *BridgeEventHandler) HandleErcToNativeAffirmationCompleted(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	recipient, ok := data["recipient"].(common.Address)
	if !ok {
		return fmt.Errorf("recipient type %T is invalid: %w", data["recipient"], ErrWrongArgumentType)
	}
	value, ok := data["value"].(*big.Int)
	if !ok {
		return fmt.Errorf("value type %T is invalid: %w", data["value"], ErrWrongArgumentType)
	}
	transactionHash, ok := data["transactionHash"].([32]byte)
	if !ok {
		return fmt.Errorf("transactionHash type %T is invalid: %w", data["transactionHash"], ErrWrongArgumentType)
	}

	valueBytes := common.BigToHash(value)

	msg := recipient[:]
	msg = append(msg, valueBytes[:]...)
	msg = append(msg, transactionHash[:]...)

	return p.repo.ExecutedMessages.Ensure(ctx, &entity.ExecutedMessage{
		LogID:     log.ID,
		BridgeID:  p.bridgeID,
		MessageID: crypto.Keccak256Hash(msg),
		Status:    true,
	})
}

func (p *BridgeEventHandler) HandleCollectedSignatures(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	msgHash, ok := data["messageHash"].([32]byte)
	if !ok {
		return fmt.Errorf("messageHash type %T is invalid: %w", data["messageHash"], ErrWrongArgumentType)
	}
	relayer, ok := data["authorityResponsibleForRelay"].(common.Address)
	if !ok {
		return fmt.Errorf("authorityResponsibleForRelay type %T is invalid: %w", data["authorityResponsibleForRelay"], ErrWrongArgumentType)
	}
	numSignatures, ok := data["NumberOfCollectedSignatures"].(*big.Int)
	if !ok {
		return fmt.Errorf("NumberOfCollectedSignatures type %T is invalid: %w", data["NumberOfCollectedSignatures"], ErrWrongArgumentType)
	}

	return p.repo.CollectedMessages.Ensure(ctx, &entity.CollectedMessage{
		LogID:             log.ID,
		BridgeID:          p.bridgeID,
		MsgHash:           msgHash,
		ResponsibleSigner: relayer,
		NumSignatures:     uint(numSignatures.Uint64()),
	})
}

func (p *BridgeEventHandler) HandleUserRequestForInformation(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	messageID, ok := data["messageId"].([32]byte)
	if !ok {
		return fmt.Errorf("messageId type %T is invalid: %w", data["messageId"], ErrWrongArgumentType)
	}
	requestSelector, ok := data["requestSelector"].([32]byte)
	if !ok {
		return fmt.Errorf("requestSelector type %T is invalid: %w", data["requestSelector"], ErrWrongArgumentType)
	}
	sender, ok := data["sender"].(common.Address)
	if !ok {
		return fmt.Errorf("sender type %T is invalid: %w", data["sender"], ErrWrongArgumentType)
	}
	requestData, ok := data["data"].([]byte)
	if !ok {
		return fmt.Errorf("data type %T is invalid: %w", data["data"], ErrWrongArgumentType)
	}

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
	messageID, ok := data["messageId"].([32]byte)
	if !ok {
		return fmt.Errorf("messageId type %T is invalid: %w", data["messageId"], ErrWrongArgumentType)
	}
	validator, ok := data["signer"].(common.Address)
	if !ok {
		return fmt.Errorf("signer type %T is invalid: %w", data["signer"], ErrWrongArgumentType)
	}

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
	messageID, ok := data["messageId"].([32]byte)
	if !ok {
		return fmt.Errorf("messageId type %T is invalid: %w", data["messageId"], ErrWrongArgumentType)
	}
	status, ok := data["status"].(bool)
	if !ok {
		return fmt.Errorf("status type %T is invalid: %w", data["status"], ErrWrongArgumentType)
	}
	callbackStatus, ok := data["callbackStatus"].(bool)
	if !ok {
		return fmt.Errorf("callbackStatus type %T is invalid: %w", data["callbackStatus"], ErrWrongArgumentType)
	}

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
	validator, ok := data["validator"].(common.Address)
	if !ok {
		return fmt.Errorf("validator type %T is invalid: %w", data["validator"], ErrWrongArgumentType)
	}
	return p.repo.BridgeValidators.Ensure(ctx, &entity.BridgeValidator{
		LogID:    log.ID,
		BridgeID: p.bridgeID,
		ChainID:  log.ChainID,
		Address:  validator,
	})
}

func (p *BridgeEventHandler) HandleValidatorRemoved(ctx context.Context, log *entity.Log, data map[string]interface{}) error {
	validator, ok := data["validator"].(common.Address)
	if !ok {
		return fmt.Errorf("validator type %T is invalid: %w", data["validator"], ErrWrongArgumentType)
	}
	val, err := p.repo.BridgeValidators.GetActiveValidator(ctx, p.bridgeID, log.ChainID, validator)
	if err != nil {
		return db.IgnoreErrNotFound(err)
	}
	val.RemovedLogID = &log.ID
	return p.repo.BridgeValidators.Ensure(ctx, val)
}
