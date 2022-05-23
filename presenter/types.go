package presenter

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/entity"
)

type MessageInfo struct {
	BridgeID  string
	MsgHash   common.Hash
	MessageID common.Hash
	Direction entity.Direction
	Sender    common.Address
	Executor  common.Address
	DataType  uint
}

type InformationRequestInfo struct {
	BridgeID  string
	MessageID common.Hash
	Direction entity.Direction
	Sender    common.Address
	Executor  common.Address
}

type ErcToNativeMessageInfo struct {
	BridgeID  string
	MsgHash   common.Hash
	Direction entity.Direction
	Sender    common.Address
	Receiver  common.Address
	Value     string
}

type EventInfo struct {
	Action         string
	LogID          uint            `json:"-"`
	Signer         *common.Address `json:",omitempty"`
	Data           hexutil.Bytes   `json:",omitempty"`
	Count          uint            `json:",omitempty"`
	Status         bool            `json:",omitempty"`
	CallbackStatus bool            `json:",omitempty"`
	*TxInfo
}

type SearchResult struct {
	Event         *EventInfo
	Message       interface{}
	RelatedEvents []*EventInfo
}

type LogResult struct {
	LogID       uint
	ChainID     string
	Address     common.Address
	Topic0      *common.Hash `json:",omitempty"`
	Topic1      *common.Hash `json:",omitempty"`
	Topic2      *common.Hash `json:",omitempty"`
	Topic3      *common.Hash `json:",omitempty"`
	Data        hexutil.Bytes
	TxHash      common.Hash
	BlockNumber uint
}

type BridgeInfo struct {
	BridgeID string
	Mode     config.BridgeMode
	Home     *BridgeSideInfo
	Foreign  *BridgeSideInfo
}

type BridgeSideInfo struct {
	Chain                  string
	ChainID                string
	BridgeAddress          common.Address
	LastFetchedBlock       uint
	LastFetchBlockTime     time.Time
	LastProcessedBlock     uint
	LastProcessedBlockTime time.Time
	Validators             []common.Address
}

type ValidatorsInfo struct {
	BridgeID   string
	Mode       config.BridgeMode
	Validators []*ValidatorInfo
}

type ValidatorInfo struct {
	Address          common.Address
	LastConfirmation *TxInfo
}

type TxInfo struct {
	BlockNumber uint
	Timestamp   time.Time
	Link        string
}

type FilterContext struct {
	ChainID   *string
	FromBlock *uint
	ToBlock   *uint
	TxHash    *common.Hash
}
