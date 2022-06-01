package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type BridgeValidator struct {
	LogID        uint           `db:"log_id"`
	BridgeID     string         `db:"bridge_id"`
	ChainID      string         `db:"chain_id"`
	Address      common.Address `db:"address"`
	RemovedLogID *uint          `db:"removed_log_id"`
	CreatedAt    *time.Time     `db:"created_at"`
	UpdatedAt    *time.Time     `db:"updated_at"`
}

type BridgeValidatorsRepo interface {
	Ensure(ctx context.Context, val *BridgeValidator) error
	GetActiveValidator(ctx context.Context, bridgeID, chainID string, address common.Address) (*BridgeValidator, error)
	FindActiveValidators(ctx context.Context, bridgeID, chainID string) ([]*BridgeValidator, error)
}
