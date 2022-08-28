package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"

	"github.com/omni/tokenbridge-monitor/db"
	"github.com/omni/tokenbridge-monitor/entity"
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

func (r *signedMessagesRepo) GetByLogID(ctx context.Context, logID uint) (*entity.SignedMessage, error) {
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
		return nil, fmt.Errorf("can't get signed messages: %w", err)
	}
	return msg, nil
}

func (r *signedMessagesRepo) FindByMsgHashes(ctx context.Context, bridgeID string, msgHashes []common.Hash) ([]*entity.SignedMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "msg_hash": msgHashes}).
		OrderBy("signer").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msgs := make([]*entity.SignedMessage, 0, 4)
	err = r.db.SelectContext(ctx, &msgs, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get signed messages: %w", err)
	}
	return msgs, nil
}

func (r *signedMessagesRepo) GetLatest(ctx context.Context, bridgeID, chainID string, signer common.Address) (*entity.SignedMessage, error) {
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
		return nil, fmt.Errorf("can't get latest signed message: %w", err)
	}
	return msg, nil
}
