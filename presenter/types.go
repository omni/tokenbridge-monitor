package presenter

import (
	"amb-monitor/entity"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type MessageInfo struct {
	MsgHash   common.Hash
	MessageID common.Hash
	Direction entity.Direction
	Sender    common.Address
	Executor  common.Address
	DataType  uint
}

type TxInfo struct {
	BlockNumber uint
	Timestamp   time.Time
	Link        string
}

type EventInfo struct {
	Event  string
	LogID  uint            `json:"-"`
	Signer *common.Address `json:",omitempty"`
	Count  uint            `json:",omitempty"`
	Status bool            `json:",omitempty"`
	*TxInfo
}

type SearchResult struct {
	Bridge  string
	Event   string
	TxHash  common.Hash
	Message *MessageInfo
	Events  []*EventInfo
}
