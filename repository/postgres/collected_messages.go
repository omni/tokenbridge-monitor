package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type collectedMessagesRepo basePostgresRepo

func NewCollectedMessagesRepo(table string, db *db.DB) entity.CollectedMessagesRepo {
	return (*collectedMessagesRepo)(newBasePostgresRepo(table, db))
}

func (r *collectedMessagesRepo) Ensure(ctx context.Context, msg *entity.CollectedMessage) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "msg_hash", "responsible_signer", "num_signatures").
		Values(msg.LogID, msg.BridgeID, msg.MsgHash, msg.ResponsibleSigner, msg.NumSignatures).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert collected message: %w", err)
	}
	return nil
}
