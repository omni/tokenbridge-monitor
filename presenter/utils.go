package presenter

import (
	"fmt"

	"github.com/poanetwork/tokenbridge-monitor/entity"
)

var formats = map[string]string{
	"1":   "https://etherscan.io/tx/%s",
	"4":   "https://rinkeby.etherscan.io/tx/%s",
	"42":  "https://kovan.etherscan.io/tx/%s",
	"56":  "https://bscscan.com/tx/%s",
	"77":  "https://blockscout.com/poa/sokol/tx/%s",
	"99":  "https://blockscout.com/poa/core/tx/%s",
	"100": "https://blockscout.com/xdai/mainnet/tx/%s",
}

func FormatLogTxLinkURL(log *entity.Log) string {
	if format, ok := formats[log.ChainID]; ok {
		return fmt.Sprintf(format, log.TransactionHash)
	}
	return log.TransactionHash.String()
}

func NewMessageInfo(msg *entity.Message) *MessageInfo {
	return &MessageInfo{
		BridgeID:  msg.BridgeID,
		MsgHash:   msg.MsgHash,
		MessageID: msg.MessageID,
		Direction: msg.Direction,
		Sender:    msg.Sender,
		Executor:  msg.Executor,
		DataType:  msg.DataType,
	}
}

func NewInformationRequestInfo(req *entity.InformationRequest) *InformationRequestInfo {
	return &InformationRequestInfo{
		BridgeID:  req.BridgeID,
		MessageID: req.MessageID,
		Direction: req.Direction,
		Sender:    req.Sender,
		Executor:  req.Executor,
	}
}

func NewErcToNativeMessageInfo(req *entity.ErcToNativeMessage) *ErcToNativeMessageInfo {
	return &ErcToNativeMessageInfo{
		BridgeID:  req.BridgeID,
		MsgHash:   req.MsgHash,
		Direction: req.Direction,
		Sender:    req.Sender,
		Receiver:  req.Receiver,
		Value:     req.Value,
	}
}
