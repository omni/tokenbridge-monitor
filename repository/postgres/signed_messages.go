package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"
)

type signedMessagesRepo struct {
	table string
	db    *db.DB
}

func NewSignedMessagesRepo(table string, db *db.DB) entity.SignedMessagesRepo {
	return &signedMessagesRepo{
		table: table,
		db:    db,
	}
}

func (r *signedMessagesRepo) Ensure(ctx context.Context, msg *entity.SignedMessage) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "msg_hash", "signer", "is_responsible").
		Values(msg.LogID, msg.BridgeID, msg.MsgHash, msg.Signer, msg.IsResponsible).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert signed message: %w", err)
	}
	return nil
}

func (r *signedMessagesRepo) MarkResponsibleSigner(ctx context.Context, bridgeID string, msgHash common.Hash, signer common.Address) error {
	q, args, err := sq.Update(r.table).
		Set("is_responsible", true).
		Where(sq.Eq{"bridge_id": bridgeID, "msg_hash": msgHash, "signer": signer}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't mark last signature: %w", err)
	}
	updates, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("can't get update result: %w", err)
	}
	if updates != 1 {
		return fmt.Errorf("expected update of 1 row, got %d updates instead", updates)
	}
	return nil
}
