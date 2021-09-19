package monitor

import (
	"amb-monitor/entity"
)

type BlocksRange struct {
	From uint
	To   uint
}

type LogsBatch struct {
	BlockNumber uint
	Logs        []*entity.Log
}
