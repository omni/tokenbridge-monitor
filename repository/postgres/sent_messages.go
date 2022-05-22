package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"

	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
)

type sentMessagesRepo basePostgresRepo

func NewSentMessagesRepo(table string, db *db.DB) entity.SentMessagesRepo {
	return (*sentMessagesRepo)(newBasePostgresRepo(table, db))
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

func (r *sentMessagesRepo) FindByLogID(ctx context.Context, logID uint) (*entity.SentMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"log_id": logID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.SentMessage)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, fmt.Errorf("can't get sent message: %w", err)
	}
	return msg, nil
}

func (r *sentMessagesRepo) FindByMsgHash(ctx context.Context, bridgeID string, msgHash common.Hash) (*entity.SentMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "msg_hash": msgHash}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.SentMessage)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, fmt.Errorf("can't get sent message: %w", err)
	}
	return msg, nil
}
