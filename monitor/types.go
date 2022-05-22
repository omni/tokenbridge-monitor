package monitor

import (
	"math"
	"tokenbridge-monitor/entity"

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
