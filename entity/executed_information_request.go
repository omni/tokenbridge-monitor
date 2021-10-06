package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type ExecutedInformationRequest struct {
	LogID          uint        `db:"log_id"`
	BridgeID       string      `db:"bridge_id"`
	MessageID      common.Hash `db:"message_id"`
	Status         bool        `db:"status"`
	CallbackStatus bool        `db:"callback_status"`
	Data           []byte      `db:"data"`
	CreatedAt      *time.Time  `db:"created_at"`
	UpdatedAt      *time.Time  `db:"updated_at"`
}

type ExecutedInformationRequestsRepo interface {
	Ensure(ctx context.Context, msg *ExecutedInformationRequest) error
}
