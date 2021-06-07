package monitor

import (
	"amb-monitor/db"
	"encoding/binary"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgtype/pgxtype"
	"github.com/jackc/pgx/v4"
)

type EventHandler func(*BridgeSideMonitor, types.Log, map[string]interface{}) (db.Executable, error)

func buildMessage(bridgeId string, direction int, encodedData []byte) *Message {
	msgHash := crypto.Keccak256Hash(encodedData).Hex()
	messageId := encodedData[:32]
	sender := encodedData[32:52]
	executor := encodedData[52:72]
	gasLimit := binary.BigEndian.Uint32(encodedData[72:76])
	dataType := encodedData[78]
	data := encodedData[79+encodedData[76]+encodedData[77]:]

	return &Message{
		BridgeId:  bridgeId,
		Direction: direction,
		MsgHash:   msgHash,
		MessageId: hexutil.Encode(messageId[:]),
		Sender:    hexutil.Encode(sender),
		Executor:  hexutil.Encode(executor),
		GasLimit:  gasLimit,
		DataType:  dataType,
		Data:      hexutil.Encode(data),
	}
}

func buildLegacyMessage(bridgeId string, direction int, encodedData []byte) *Message {
	msgHash := crypto.Keccak256Hash(encodedData).Hex()
	messageId := encodedData[:32]
	sender := encodedData[32:52]
	executor := encodedData[52:72]
	gasLimit := binary.BigEndian.Uint32(encodedData[100:104])
	dataType := encodedData[104]
	if dataType > 0 {
		panic("unsupported datatype")
	}
	data := encodedData[105:]

	return &Message{
		BridgeId:  bridgeId,
		Direction: direction,
		MsgHash:   msgHash,
		MessageId: hexutil.Encode(messageId[:]),
		Sender:    hexutil.Encode(sender),
		Executor:  hexutil.Encode(executor),
		GasLimit:  gasLimit,
		DataType:  dataType,
		Data:      hexutil.Encode(data),
	}
}

func buildTxLogInfo(state *BridgeSideMonitor, log types.Log) *TxLogInfo {
	return &TxLogInfo{
		ChainId:     state.ChainId,
		TxHash:      hexutil.Encode(log.TxHash[:]),
		BlockNumber: log.BlockNumber,
		LogIndex:    log.Index,
	}
}

func HandleUserRequestForAffirmation(state *BridgeSideMonitor, log types.Log, event map[string]interface{}) (db.Executable, error) {
	encodedData := event["encodedData"].([]byte)
	message := buildMessage(state.parent.Id, ForeignToHome, encodedData)
	messageRequest := &MessageRequest{
		Message:   message,
		TxLogInfo: buildTxLogInfo(state, log),
	}

	return db.ExecFuncAtomic(func(q pgxtype.Querier) error {
		err := message.Insert(q)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		return messageRequest.Insert(q)
	}), nil
}

func HandleLegacyUserRequestForAffirmation(state *BridgeSideMonitor, log types.Log, event map[string]interface{}) (db.Executable, error) {
	encodedData := event["encodedData"].([]byte)
	encodedData = append(log.TxHash[:], encodedData...)
	message := buildLegacyMessage(state.parent.Id, ForeignToHome, encodedData)
	messageRequest := &MessageRequest{
		Message:   message,
		TxLogInfo: buildTxLogInfo(state, log),
	}

	return db.ExecFuncAtomic(func(q pgxtype.Querier) error {
		err := message.Insert(q)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		return messageRequest.Insert(q)
	}), nil
}

func HandleUserRequestForSignature(state *BridgeSideMonitor, log types.Log, event map[string]interface{}) (db.Executable, error) {
	encodedData := event["encodedData"].([]byte)
	message := buildMessage(state.parent.Id, HomeToForeign, encodedData)
	messageRequest := &MessageRequest{
		Message:   message,
		TxLogInfo: buildTxLogInfo(state, log),
	}

	return db.ExecFuncAtomic(func(q pgxtype.Querier) error {
		err := message.Insert(q)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		return messageRequest.Insert(q)
	}), nil
}

func HandleLegacyUserRequestForSignature(state *BridgeSideMonitor, log types.Log, event map[string]interface{}) (db.Executable, error) {
	encodedData := event["encodedData"].([]byte)
	encodedData = append(log.TxHash[:], encodedData...)
	message := buildLegacyMessage(state.parent.Id, HomeToForeign, encodedData)
	messageRequest := &MessageRequest{
		Message:   message,
		TxLogInfo: buildTxLogInfo(state, log),
	}

	return db.ExecFuncAtomic(func(q pgxtype.Querier) error {
		err := message.Insert(q)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		return messageRequest.Insert(q)
	}), nil
}

func HandleSignedForUserRequest(state *BridgeSideMonitor, log types.Log, event map[string]interface{}) (db.Executable, error) {
	msgHash := event["messageHash"].([32]byte)
	validator := event["signer"].(common.Address)

	message := &Message{
		BridgeId: state.parent.Id,
		MsgHash:  hexutil.Encode(msgHash[:]),
	}
	confirmation := &MessageConfirmation{
		Message:   message,
		TxLogInfo: buildTxLogInfo(state, log),
		Validator: hexutil.Encode(validator[:]),
	}

	return db.ExecFuncAtomic(confirmation.Insert), nil
}

func HandleSignedForAffirmation(state *BridgeSideMonitor, log types.Log, event map[string]interface{}) (db.Executable, error) {
	msgHash := event["messageHash"].([32]byte)
	validator := event["signer"].(common.Address)

	message := &Message{
		BridgeId: state.parent.Id,
		MsgHash:  hexutil.Encode(msgHash[:]),
	}
	confirmation := &MessageConfirmation{
		Message:   message,
		TxLogInfo: buildTxLogInfo(state, log),
		Validator: hexutil.Encode(validator[:]),
	}

	return db.ExecFuncAtomic(confirmation.Insert), nil
}

func HandleRelayedMessage(state *BridgeSideMonitor, log types.Log, event map[string]interface{}) (db.Executable, error) {
	messageId := event["messageId"].([32]byte)
	status := event["status"].(bool)

	message := &Message{
		BridgeId:  state.parent.Id,
		MessageId: hexutil.Encode(messageId[:]),
	}
	execution := &MessageExecution{
		Message:   message,
		TxLogInfo: buildTxLogInfo(state, log),
		Status:    status,
	}

	return db.ExecFuncAtomic(execution.Insert), nil
}

func HandleAffirmationCompleted(state *BridgeSideMonitor, log types.Log, event map[string]interface{}) (db.Executable, error) {
	messageId := event["messageId"].([32]byte)
	status := event["status"].(bool)

	message := &Message{
		BridgeId:  state.parent.Id,
		MessageId: hexutil.Encode(messageId[:]),
	}
	execution := &MessageExecution{
		Message:   message,
		TxLogInfo: buildTxLogInfo(state, log),
		Status:    status,
	}

	return db.ExecFuncAtomic(execution.Insert), nil
}
