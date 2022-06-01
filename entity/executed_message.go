package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type ExecutedMessage struct {
	LogID     uint        `db:"log_id"`
	BridgeID  string      `db:"bridge_id"`
	MessageID common.Hash `db:"message_id"`
	Status    bool        `db:"status"`
	CreatedAt *time.Time  `db:"created_at"`
	UpdatedAt *time.Time  `db:"updated_at"`
}

type ExecutedMessagesRepo interface {
	Ensure(ctx context.Context, msg *ExecutedMessage) error
	GetByLogID(ctx context.Context, logID uint) (*ExecutedMessage, error)
	GetByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) (*ExecutedMessage, error)
}
