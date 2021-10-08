package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type executedInformationRequestsRepo basePostgresRepo

func NewExecutedInformationRequestsRepo(table string, db *db.DB) entity.ExecutedInformationRequestsRepo {
	return (*executedInformationRequestsRepo)(newBasePostgresRepo(table, db))
}

func (r *executedInformationRequestsRepo) Ensure(ctx context.Context, msg *entity.ExecutedInformationRequest) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "message_id", "status", "callback_status", "data").
		Values(msg.LogID, msg.BridgeID, msg.MessageID, msg.Status, msg.CallbackStatus, msg.Data).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert executed information request: %w", err)
	}
	return nil
}
