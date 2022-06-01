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

type LogInfo struct {
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

func NewLogInfo(log *entity.Log) *LogInfo {
	return &LogInfo{
		LogID:       log.ID,
		ChainID:     log.ChainID,
		Address:     log.Address,
		Topic0:      log.Topic0,
		Topic1:      log.Topic1,
		Topic2:      log.Topic2,
		Topic3:      log.Topic3,
		Data:        log.Data,
		TxHash:      log.TransactionHash,
		BlockNumber: log.BlockNumber,
	}
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
