package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type SignedMessage struct {
	LogID     uint           `db:"log_id"`
	BridgeID  string         `db:"bridge_id"`
	MsgHash   common.Hash    `db:"msg_hash"`
	Signer    common.Address `db:"signer"`
	CreatedAt *time.Time     `db:"created_at"`
	UpdatedAt *time.Time     `db:"updated_at"`
}

type SignedMessagesRepo interface {
	Ensure(ctx context.Context, msg *SignedMessage) error
	GetByLogID(ctx context.Context, logID uint) (*SignedMessage, error)
	FindByMsgHash(ctx context.Context, bridgeID string, msgHash []common.Hash) ([]*SignedMessage, error)
	GetLatest(ctx context.Context, bridgeID, chainID string, signer common.Address) (*SignedMessage, error)
}
