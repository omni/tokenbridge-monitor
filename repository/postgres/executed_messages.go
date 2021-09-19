package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type executedMessagesRepo struct {
	table string
	db    *db.DB
}

func NewExecutedMessagesRepo(table string, db *db.DB) entity.ExecutedMessagesRepo {
	return &executedMessagesRepo{
		table: table,
		db:    db,
	}
}

func (r *executedMessagesRepo) Ensure(ctx context.Context, msg *entity.ExecutedMessage) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "message_id", "status").
		Values(msg.LogID, msg.BridgeID, msg.MessageID, msg.Status).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert executed message: %w", err)
	}
	return nil
}
