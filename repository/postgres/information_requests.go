package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type informationRequestsRepo basePostgresRepo

func NewInformationRequestsRepo(table string, db *db.DB) entity.InformationRequestsRepo {
	return (*informationRequestsRepo)(newBasePostgresRepo(table, db))
}

func (r *informationRequestsRepo) Ensure(ctx context.Context, msg *entity.InformationRequest) error {
	q, args, err := sq.Insert(r.table).
		Columns("bridge_id", "message_id", "direction", "request_selector", "sender", "executor", "data").
		Values(msg.BridgeID, msg.MessageID, msg.Direction, msg.RequestSelector, msg.Sender, msg.Executor, msg.Data).
		Suffix("ON CONFLICT (bridge_id, message_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert information request: %w", err)
	}
	return nil
}
