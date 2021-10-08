package entity

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type CollectedMessage struct {
	LogID             uint           `db:"log_id"`
	BridgeID          string         `db:"bridge_id"`
	MsgHash           common.Hash    `db:"msg_hash"`
	ResponsibleSigner common.Address `db:"responsible_signer"`
	NumSignatures     uint           `db:"num_signatures"`
	CreatedAt         *time.Time     `db:"created_at"`
	UpdatedAt         *time.Time     `db:"updated_at"`
}

type CollectedMessagesRepo interface {
	Ensure(ctx context.Context, msg *CollectedMessage) error
	FindByMsgHash(ctx context.Context, bridgeID string, msgHash common.Hash) (*CollectedMessage, error)
}
