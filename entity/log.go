package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Log struct {
	ID              uint           `db:"id"`
	ChainID         string         `db:"chain_id"`
	Address         common.Address `db:"address"`
	Topic0          *common.Hash   `db:"topic0"`
	Topic1          *common.Hash   `db:"topic1"`
	Topic2          *common.Hash   `db:"topic2"`
	Topic3          *common.Hash   `db:"topic3"`
	Data            []byte         `db:"data"`
	BlockNumber     uint           `db:"block_number"`
	LogIndex        uint           `db:"log_index"`
	TransactionHash common.Hash    `db:"transaction_hash"`
	CreatedAt       *time.Time     `db:"created_at"`
	UpdatedAt       *time.Time     `db:"updated_at"`
}

type LogsRepo interface {
	Ensure(ctx context.Context, logs ...*Log) error
	GetByID(ctx context.Context, id uint) (*Log, error)
	FindByBlockRange(ctx context.Context, chainID string, addr []common.Address, fromBlock, toBlock uint) ([]*Log, error)
	FindByBlockNumber(ctx context.Context, chainID string, block uint) ([]*Log, error)
	FindByTxHash(ctx context.Context, txHash common.Hash) ([]*Log, error)
}

func NewLog(chainID string, log types.Log) *Log {
	e := &Log{
		ChainID:         chainID,
		Address:         log.Address,
		Data:            log.Data,
		BlockNumber:     uint(log.BlockNumber),
		LogIndex:        log.Index,
		TransactionHash: log.TxHash,
	}
	if len(log.Topics) > 0 {
		e.Topic0 = &log.Topics[0]
		if len(log.Topics) > 1 {
			e.Topic1 = &log.Topics[1]
			if len(log.Topics) > 2 {
				e.Topic2 = &log.Topics[2]
				if len(log.Topics) > 3 {
					e.Topic3 = &log.Topics[3]
				}
			}
		}
	}
	return e
}
