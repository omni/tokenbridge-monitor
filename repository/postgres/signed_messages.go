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

type signedMessagesRepo basePostgresRepo

func NewSignedMessagesRepo(table string, db *db.DB) entity.SignedMessagesRepo {
	return (*signedMessagesRepo)(newBasePostgresRepo(table, db))
}

func (r *signedMessagesRepo) Ensure(ctx context.Context, msg *entity.SignedMessage) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "msg_hash", "signer").
		Values(msg.LogID, msg.BridgeID, msg.MsgHash, msg.Signer).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert signed message: %w", err)
	}
	return nil
}

func (r *signedMessagesRepo) FindByLogID(ctx context.Context, logID uint) (*entity.SignedMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"log_id": logID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.SignedMessage)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get signed messages: %w", err)
	}
	return msg, nil
}

func (r *signedMessagesRepo) FindByMsgHash(ctx context.Context, bridgeID string, msgHash common.Hash) ([]*entity.SignedMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "msg_hash": msgHash}).
		OrderBy("signer").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msgs := make([]*entity.SignedMessage, 0, 4)
	err = r.db.SelectContext(ctx, &msgs, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get signed messages: %w", err)
	}
	return msgs, nil
}

func (r *signedMessagesRepo) FindLatest(ctx context.Context, bridgeID, chainID string, signer common.Address) (*entity.SignedMessage, error) {
	q, args, err := sq.Select(r.table + ".*").
		From(r.table).
		Join("logs l ON l.id = log_id").
		Where(sq.Eq{"bridge_id": bridgeID, "signer": signer, "l.chain_id": chainID}).
		OrderBy("l.block_number DESC").
		Limit(1).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.SignedMessage)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get latest signed message: %w", err)
	}
	return msg, nil
}
