package monitor

import (
	"amb-monitor/entity"
	"bytes"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
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
	dataType := encodedData[104]
	if dataType > 0 {
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
