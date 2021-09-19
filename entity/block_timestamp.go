package entity

import (
	"context"
	"time"
)

type BlockTimestamp struct {
	ChainID     string     `db:"chain_id"`
	BlockNumber uint       `db:"block_number"`
	Timestamp   time.Time  `db:"timestamp"`
	CreatedAt   *time.Time `db:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at"`
}

type BlockTimestampsRepo interface {
	Ensure(ctx context.Context, cursor *BlockTimestamp) error
}
