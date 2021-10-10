package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type SignedInformationRequest struct {
	LogID     uint           `db:"log_id"`
	BridgeID  string         `db:"bridge_id"`
	MessageID common.Hash    `db:"message_id"`
	Signer    common.Address `db:"signer"`
	Data      []byte         `db:"data"`
	CreatedAt *time.Time     `db:"created_at"`
	UpdatedAt *time.Time     `db:"updated_at"`
}

type SignedInformationRequestsRepo interface {
	Ensure(ctx context.Context, msg *SignedInformationRequest) error
	FindByLogID(ctx context.Context, logID uint) (*SignedInformationRequest, error)
	FindByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) ([]*SignedInformationRequest, error)
}
