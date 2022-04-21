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

type ercToNativeMessagesRepo basePostgresRepo

func NewErcToNativeMessagesRepo(table string, db *db.DB) entity.ErcToNativeMessagesRepo {
	return (*ercToNativeMessagesRepo)(newBasePostgresRepo(table, db))
}

func (r *ercToNativeMessagesRepo) Ensure(ctx context.Context, msg *entity.ErcToNativeMessage) error {
	q, args, err := sq.Insert(r.table).
		Columns("bridge_id", "msg_hash", "direction", "sender", "receiver", "value").
		Values(msg.BridgeID, msg.MsgHash, msg.Direction, msg.Sender, msg.Receiver, msg.Value).
		Suffix("ON CONFLICT (bridge_id, msg_hash) DO UPDATE SET sender = EXCLUDED.sender, updated_at = NOW()").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert message: %w", err)
	}
	return nil
}

func (r *ercToNativeMessagesRepo) FindByMsgHash(ctx context.Context, bridgeID string, msgHash common.Hash) (*entity.ErcToNativeMessage, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "msg_hash": msgHash}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.ErcToNativeMessage)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't get message: %w", err)
	}
	return msg, nil
}
