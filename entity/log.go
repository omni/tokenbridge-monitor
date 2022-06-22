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

type LogsFilter struct {
	ChainID    *string
	Addresses  []common.Address
	FromBlock  *uint
	ToBlock    *uint
	TxHash     *common.Hash
	Topic0     []common.Hash
	Topic1     []common.Hash
	Topic2     []common.Hash
	Topic3     []common.Hash
	DataLength *uint
}

type LogsRepo interface {
	Ensure(ctx context.Context, logs ...*Log) error
	GetByID(ctx context.Context, id uint) (*Log, error)
	Find(ctx context.Context, filter LogsFilter) ([]*Log, error)
}

func NewLog(chainID string, log types.Log) *Log {
	res := &Log{
		ChainID:         chainID,
		Address:         log.Address,
		Data:            log.Data,
		BlockNumber:     uint(log.BlockNumber),
		LogIndex:        log.Index,
		TransactionHash: log.TxHash,
	}
	//nolint:nestif
	if len(log.Topics) > 0 {
		res.Topic0 = &log.Topics[0]
		if len(log.Topics) > 1 {
			res.Topic1 = &log.Topics[1]
			if len(log.Topics) > 2 {
				res.Topic2 = &log.Topics[2]
				if len(log.Topics) > 3 {
					res.Topic3 = &log.Topics[3]
				}
			}
		}
	}
	return res
}

func (l *Log) Topics() []common.Hash {
	topics := make([]common.Hash, 0, 4)
	//nolint:nestif
	if l.Topic0 != nil {
		topics = append(topics, *l.Topic0)
		if l.Topic1 != nil {
			topics = append(topics, *l.Topic1)
			if l.Topic2 != nil {
				topics = append(topics, *l.Topic2)
				if l.Topic3 != nil {
					topics = append(topics, *l.Topic3)
				}
			}
		}
	}
	return topics
}
