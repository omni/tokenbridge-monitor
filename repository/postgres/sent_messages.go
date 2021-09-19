package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type sentMessagesRepo struct {
	table string
	db    *db.DB
}

func NewSentMessagesRepo(table string, db *db.DB) entity.SentMessagesRepo {
	return &sentMessagesRepo{
		table: table,
		db:    db,
	}
}

func (r *sentMessagesRepo) Ensure(ctx context.Context, msg *entity.SentMessage) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "msg_hash").
		Values(msg.LogID, msg.BridgeID, msg.MsgHash).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert sent message: %w", err)
	}
	return nil
}
