package monitor

import (
	"amb-monitor/entity"

	"github.com/ethereum/go-ethereum/common"
)

type BlocksRange struct {
	From  uint
	To    uint
	Topic *common.Hash
}

type LogsBatch struct {
	BlockNumber uint
	Logs        []*entity.Log
}
