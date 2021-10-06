package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type signedInformationRequestsRepo struct {
	table string
	db    *db.DB
}

func NewSignedInformationRequestsRepo(table string, db *db.DB) entity.SignedInformationRequestsRepo {
	return &signedInformationRequestsRepo{
		table: table,
		db:    db,
	}
}

func (r *signedInformationRequestsRepo) Ensure(ctx context.Context, msg *entity.SignedInformationRequest) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "message_id", "signer", "data").
		Values(msg.LogID, msg.BridgeID, msg.MessageID, msg.Signer, msg.Data).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert signed information request: %w", err)
	}
	return nil
}
