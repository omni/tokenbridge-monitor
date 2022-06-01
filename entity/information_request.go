package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type InformationRequest struct {
	ID              uint           `db:"id"`
	BridgeID        string         `db:"bridge_id"`
	MessageID       common.Hash    `db:"message_id"`
	Direction       Direction      `db:"direction"`
	RequestSelector common.Hash    `db:"request_selector"`
	Sender          common.Address `db:"sender"`
	Executor        common.Address `db:"executor"`
	Data            []byte         `db:"data"`
	CreatedAt       *time.Time     `db:"created_at"`
	UpdatedAt       *time.Time     `db:"updated_at"`
}

type InformationRequestsRepo interface {
	Ensure(ctx context.Context, msg *InformationRequest) error
	GetByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) (*InformationRequest, error)
}
