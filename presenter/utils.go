package presenter

import (
	"amb-monitor/entity"
	"fmt"
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

func logToTxLink(log *entity.Log) string {
	if format, ok := formats[log.ChainID]; ok {
		return fmt.Sprintf(format, log.TransactionHash)
	}
	return log.TransactionHash.String()
}

func messageToMessageInfo(msg *entity.Message) *MessageInfo {
	return &MessageInfo{
		MsgHash:   msg.MsgHash,
		MessageID: msg.MessageID,
		Direction: msg.Direction,
		Sender:    msg.Sender,
		Executor:  msg.Executor,
		DataType:  msg.DataType,
	}
}
