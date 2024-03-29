package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"

	"github.com/omni/tokenbridge-monitor/db"
	"github.com/omni/tokenbridge-monitor/entity"
)

type executedMessagesRepo basePostgresRepo

func NewExecutedMessagesRepo(table string, db *db.DB) entity.ExecutedMessagesRepo {
	return (*executedMessagesRepo)(newBasePostgresRepo(table, db))
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

func (r *executedMessagesRepo) GetByLogID(ctx context.Context, logID uint) (*entity.ExecutedMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"log_id": logID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.ExecutedMessage)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get executed message: %w", err)
	}
	return msg, nil
}

func (r *executedMessagesRepo) GetByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) (*entity.ExecutedMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "message_id": messageID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.ExecutedMessage)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get executed message: %w", err)
	}
	return msg, nil
}
