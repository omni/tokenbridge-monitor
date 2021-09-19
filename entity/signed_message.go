package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type SignedMessage struct {
	LogID         uint           `db:"log_id"`
	BridgeID      string         `db:"bridge_id"`
	MsgHash       common.Hash    `db:"msg_hash"`
	Signer        common.Address `db:"signer"`
	IsResponsible bool           `db:"is_responsible"`
	CreatedAt     *time.Time     `db:"created_at"`
	UpdatedAt     *time.Time     `db:"updated_at"`
}

type SignedMessagesRepo interface {
	Ensure(ctx context.Context, msg *SignedMessage) error
	MarkResponsibleSigner(ctx context.Context, bridgeID string, msgHash common.Hash, signer common.Address) error
}
