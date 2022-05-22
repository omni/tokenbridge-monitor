package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tokenbridge-monitor/db"
	"tokenbridge-monitor/entity"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"
)

type sentInformationRequestsRepo basePostgresRepo

func NewSentInformationRequestsRepo(table string, db *db.DB) entity.SentInformationRequestsRepo {
	return (*sentInformationRequestsRepo)(newBasePostgresRepo(table, db))
}

func (r *sentInformationRequestsRepo) Ensure(ctx context.Context, msg *entity.SentInformationRequest) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "message_id").
		Values(msg.LogID, msg.BridgeID, msg.MessageID).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert sent information request: %w", err)
	}
	return nil
}

func (r *sentInformationRequestsRepo) FindByLogID(ctx context.Context, logID uint) (*entity.SentInformationRequest, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"log_id": logID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	req := new(entity.SentInformationRequest)
	err = r.db.GetContext(ctx, req, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, fmt.Errorf("can't get sent information requesst: %w", err)
	}
	return req, nil
}

func (r *sentInformationRequestsRepo) FindByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) (*entity.SentInformationRequest, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "message_id": messageID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	req := new(entity.SentInformationRequest)
	err = r.db.GetContext(ctx, req, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, fmt.Errorf("can't get sent information request: %w", err)
	}
	return req, nil
}
