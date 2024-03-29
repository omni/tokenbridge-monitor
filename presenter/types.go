package presenter

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/omni/tokenbridge-monitor/config"
	"github.com/omni/tokenbridge-monitor/contract/bridgeabi"
	"github.com/omni/tokenbridge-monitor/entity"
)

type MessageInfo struct {
	BridgeID  string
	MsgHash   common.Hash
	MessageID common.Hash
	Direction entity.Direction
	Sender    common.Address
	Executor  common.Address
	DataType  uint
	Data      hexutil.Bytes
}

type InformationRequestInfo struct {
	BridgeID  string
	MessageID common.Hash
	Direction entity.Direction
	Sender    common.Address
	Executor  common.Address
	Method    string
	Data      hexutil.Bytes
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

type UnsignedMessagesInfo struct {
	RequiredSignatures    uint
	ActiveValidators      []common.Address
	TotalPendingMessages  uint
	TotalUnsignedMessages uint
	UnsignedMessages      []*UnsignedMessageInfo
}

type UnsignedMessageInfo struct {
	Message        interface{}
	Link           string
	Signers        []common.Address
	MissingSigners []common.Address
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
		Data:      msg.Data,
	}
}

func decodeRequestSelector(selector common.Hash) string {
	if decoded, ok := bridgeabi.ArbitraryMessageSelectors[selector]; ok {
		return decoded
	}
	return selector.String()
}

func NewInformationRequestInfo(req *entity.InformationRequest) *InformationRequestInfo {
	return &InformationRequestInfo{
		BridgeID:  req.BridgeID,
		MessageID: req.MessageID,
		Direction: req.Direction,
		Sender:    req.Sender,
		Executor:  req.Executor,
		Method:    decodeRequestSelector(req.RequestSelector),
		Data:      req.Data,
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

func NewBridgeMessageInfo(req entity.BridgeMessage) interface{} {
	switch msg := req.(type) {
	case *entity.Message:
		return NewMessageInfo(msg)
	case *entity.ErcToNativeMessage:
		return NewErcToNativeMessageInfo(msg)
	default:
		return nil
	}
}
