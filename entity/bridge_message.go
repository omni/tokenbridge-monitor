package entity

import "github.com/ethereum/go-ethereum/common"

type BridgeMessage interface {
	GetMsgHash() common.Hash
	GetMessageID() common.Hash
	GetDirection() Direction
	GetRawMessage() []byte
}

func ToBridgeMessages(v interface{}) []BridgeMessage {
	switch msgs := v.(type) {
	case []*Message:
		res := make([]BridgeMessage, len(msgs))
		for i, msg := range msgs {
			res[i] = msg
		}
		return res
	case []*ErcToNativeMessage:
		res := make([]BridgeMessage, len(msgs))
		for i, msg := range msgs {
			res[i] = msg
		}
		return res
	default:
		return nil
	}
}
