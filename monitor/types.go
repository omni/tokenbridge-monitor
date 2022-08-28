package monitor

import (
	"errors"
	"math"

	"github.com/omni/tokenbridge-monitor/entity"
)

var ErrWrongArgumentType = errors.New("argument has unexpected type")

type BlocksRange struct {
	From uint
	To   uint
}

type LogsBatch struct {
	BlockNumber uint
	Logs        []*entity.Log
}

func SplitBlockRange(fromBlock uint, toBlock uint, maxSize uint) []*BlocksRange {
	batches := make([]*BlocksRange, 0, 10)
	for fromBlock <= toBlock {
		batchToBlock := fromBlock + maxSize - 1
		if batchToBlock > toBlock {
			batchToBlock = toBlock
		}
		batches = append(batches, &BlocksRange{
			From: fromBlock,
			To:   batchToBlock,
		})
		fromBlock += maxSize
	}
	return batches
}

func SplitLogsInBatches(logs []*entity.Log) []*LogsBatch {
	batches := make([]*LogsBatch, 0, 10)
	// fake log to simplify loop, it will be skipped
	logs = append(logs, &entity.Log{BlockNumber: math.MaxUint32})
	batchStartIndex := 0
	for i, log := range logs {
		if log.BlockNumber > logs[batchStartIndex].BlockNumber {
			batches = append(batches, &LogsBatch{
				BlockNumber: logs[batchStartIndex].BlockNumber,
				Logs:        logs[batchStartIndex:i],
			})
			batchStartIndex = i
		}
	}
	return batches
}
