package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"
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

func (r *informationRequestsRepo) FindByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) (*entity.InformationRequest, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "message_id": messageID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	req := new(entity.InformationRequest)
	err = r.db.GetContext(ctx, req, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get information request: %w", err)
	}
	return req, nil
}