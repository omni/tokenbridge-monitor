package monitor

import (
	"bytes"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/poanetwork/tokenbridge-monitor/entity"
)

func unmarshalMessage(bridgeID string, direction entity.Direction, encodedData []byte) *entity.Message {
	messageID := common.BytesToHash(encodedData[:32])
	if bytes.Equal(messageID[0:4], []byte{0, 4, 0, 0}) {
		return &entity.Message{
			BridgeID:  bridgeID,
			Direction: direction,
			MsgHash:   crypto.Keccak256Hash(encodedData),
			MessageID: messageID,
			Sender:    common.BytesToAddress(encodedData[64:84]),
			Executor:  common.BytesToAddress(encodedData[84:104]),
			GasLimit:  uint(binary.BigEndian.Uint32(encodedData[104:108])),
			DataType:  uint(encodedData[108]),
			Data:      encodedData[108:],
		}
	}
	if bytes.Equal(messageID[0:4], []byte{0, 5, 0, 0}) {
		return &entity.Message{
			BridgeID:  bridgeID,
			Direction: direction,
			MsgHash:   crypto.Keccak256Hash(encodedData),
			MessageID: messageID,
			Sender:    common.BytesToAddress(encodedData[32:52]),
			Executor:  common.BytesToAddress(encodedData[52:72]),
			GasLimit:  uint(binary.BigEndian.Uint32(encodedData[72:76])),
			DataType:  uint(encodedData[78]),
			Data:      encodedData[79+encodedData[76]+encodedData[77]:],
		}
	}
	panic("unsupported message version prefix")
}

func unmarshalLegacyMessage(bridgeID string, direction entity.Direction, encodedData []byte) *entity.Message {
	// transaction hash (32 bytes) + sender (20 bytes) + receiver (20 bytes) + gas limit (32 bytes) + data type (1 byte) + calldata
	if encodedData[104] > 0 {
		panic("unsupported datatype")
	}

	return &entity.Message{
		BridgeID:  bridgeID,
		Direction: direction,
		MsgHash:   crypto.Keccak256Hash(encodedData),
		MessageID: common.BytesToHash(encodedData[:32]),
		Sender:    common.BytesToAddress(encodedData[32:52]),
		Executor:  common.BytesToAddress(encodedData[52:72]),
		GasLimit:  uint(binary.BigEndian.Uint32(encodedData[100:104])),
		DataType:  0,
		Data:      encodedData[105:],
	}
}

func unmarshalConfirmInformationResult(calldata []byte) []byte {
	// selector(4 bytes) + message id (32 bytes) + status (32 bytes) + result calldata ptr (32 bytes)
	resultPtr := 4 + binary.BigEndian.Uint32(calldata[96:100])
	resultLen := binary.BigEndian.Uint32(calldata[resultPtr+28 : resultPtr+32])
	return calldata[resultPtr+32 : resultPtr+32+resultLen]
}
