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

func (r *executedInformationRequestsRepo) FindByLogID(ctx context.Context, logID uint) (*entity.ExecutedInformationRequest, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"log_id": logID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	req := new(entity.ExecutedInformationRequest)
	err = r.db.GetContext(ctx, req, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get executed information requesst: %w", err)
	}
	return req, nil
}

func (r *executedInformationRequestsRepo) FindByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) (*entity.ExecutedInformationRequest, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "message_id": messageID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	req := new(entity.ExecutedInformationRequest)
	err = r.db.GetContext(ctx, req, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get executed information request: %w", err)
	}
	return req, nil
}
