package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type LogsCursor struct {
	ChainID            string         `db:"chain_id"`
	Address            common.Address `db:"address"`
	LastFetchedBlock   uint           `db:"last_fetched_block"`
	LastProcessedBlock uint           `db:"last_processed_block"`
	CreatedAt          *time.Time     `db:"created_at"`
	UpdatedAt          *time.Time     `db:"updated_at"`
}

type LogsCursorsRepo interface {
	Ensure(ctx context.Context, cursor *LogsCursor) error
	GetByChainIDAndAddress(ctx context.Context, chainID string, addr common.Address) (*LogsCursor, error)
}
