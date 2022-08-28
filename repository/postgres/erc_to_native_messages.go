package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"

	"github.com/omni/tokenbridge-monitor/db"
	"github.com/omni/tokenbridge-monitor/entity"
)

type ercToNativeMessagesRepo basePostgresRepo

func NewErcToNativeMessagesRepo(table string, db *db.DB) entity.ErcToNativeMessagesRepo {
	return (*ercToNativeMessagesRepo)(newBasePostgresRepo(table, db))
}

func (r *ercToNativeMessagesRepo) Ensure(ctx context.Context, msg *entity.ErcToNativeMessage) error {
	q, args, err := sq.Insert(r.table).
		Columns("bridge_id", "msg_hash", "direction", "sender", "receiver", "value", "raw_message").
		Values(msg.BridgeID, msg.MsgHash, msg.Direction, msg.Sender, msg.Receiver, msg.Value, msg.RawMessage).
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

func (r *ercToNativeMessagesRepo) GetByMsgHash(ctx context.Context, bridgeID string, msgHash common.Hash) (*entity.ErcToNativeMessage, error) {
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
		return nil, fmt.Errorf("can't get message: %w", err)
	}
	return msg, nil
}

func (r *ercToNativeMessagesRepo) FindPendingMessages(ctx context.Context, bridgeID string) ([]*entity.ErcToNativeMessage, error) {
	q, args, err := sq.Select("m.*").
		From(r.table + " m").
		LeftJoin("executed_messages em ON em.message_id = m.msg_hash AND em.bridge_id = m.bridge_id").
		Where(sq.Eq{"m.bridge_id": bridgeID, "em.log_id": nil}).
		OrderBy("m.created_at").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msgs := make([]*entity.ErcToNativeMessage, 0, 10)
	err = r.db.SelectContext(ctx, &msgs, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't find messages: %w", err)
	}
	return msgs, nil
}
