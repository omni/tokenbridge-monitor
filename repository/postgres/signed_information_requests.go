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

type signedInformationRequestsRepo basePostgresRepo

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

func (r *signedInformationRequestsRepo) FindByLogID(ctx context.Context, logID uint) (*entity.SignedInformationRequest, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"log_id": logID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	req := new(entity.SignedInformationRequest)
	err = r.db.GetContext(ctx, req, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get signed information requesst: %w", err)
	}
	return req, nil
}

func (r *signedInformationRequestsRepo) FindByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) ([]*entity.SignedInformationRequest, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "message_id": messageID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	reqs := make([]*entity.SignedInformationRequest, 0, 4)
	err = r.db.SelectContext(ctx, &reqs, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get signed information requests: %w", err)
	}
	return reqs, nil
}
