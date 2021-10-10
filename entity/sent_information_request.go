package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type SentInformationRequest struct {
	LogID     uint        `db:"log_id"`
	BridgeID  string      `db:"bridge_id"`
	MessageID common.Hash `db:"message_id"`
	CreatedAt *time.Time  `db:"created_at"`
	UpdatedAt *time.Time  `db:"updated_at"`
}

type SentInformationRequestsRepo interface {
	Ensure(ctx context.Context, msg *SentInformationRequest) error
	FindByLogID(ctx context.Context, logID uint) (*SentInformationRequest, error)
	FindByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) (*SentInformationRequest, error)
}
