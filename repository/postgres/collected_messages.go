package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"

	"github.com/omni/tokenbridge-monitor/db"
	"github.com/omni/tokenbridge-monitor/entity"
)

type collectedMessagesRepo basePostgresRepo

func NewCollectedMessagesRepo(table string, db *db.DB) entity.CollectedMessagesRepo {
	return (*collectedMessagesRepo)(newBasePostgresRepo(table, db))
}

func (r *collectedMessagesRepo) Ensure(ctx context.Context, msg *entity.CollectedMessage) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "msg_hash", "responsible_signer", "num_signatures").
		Values(msg.LogID, msg.BridgeID, msg.MsgHash, msg.ResponsibleSigner, msg.NumSignatures).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert collected message: %w", err)
	}
	return nil
}

func (r *collectedMessagesRepo) GetByMsgHash(ctx context.Context, bridgeID string, msgHash common.Hash) (*entity.CollectedMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "msg_hash": msgHash}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.CollectedMessage)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get collected message: %w", err)
	}
	return msg, nil
}
