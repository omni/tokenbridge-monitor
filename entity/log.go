package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
	FindByBlockRange(ctx context.Context, chainID string, fromBlock, toBlock uint) ([]*Log, error)
}
