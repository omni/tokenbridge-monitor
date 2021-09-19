package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type SentMessage struct {
	LogID     uint        `db:"log_id"`
	BridgeID  string      `db:"bridge_id"`
	MsgHash   common.Hash `db:"msg_hash"`
	CreatedAt *time.Time  `db:"created_at"`
	UpdatedAt *time.Time  `db:"updated_at"`
}

type SentMessagesRepo interface {
	Ensure(ctx context.Context, msg *SentMessage) error
}
