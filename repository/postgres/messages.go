package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type messagesRepo struct {
	table string
	db    *db.DB
}

func NewMessagesRepo(table string, db *db.DB) entity.MessagesRepo {
	return &messagesRepo{
		table: table,
		db:    db,
	}
}

func (r *messagesRepo) Ensure(ctx context.Context, msg *entity.Message) error {
	q, args, err := sq.Insert(r.table).
		Columns("bridge_id", "msg_hash", "message_id", "direction", "sender", "executor", "data", "data_type", "gas_limit").
		Values(msg.BridgeID, msg.MsgHash, msg.MessageID, msg.Direction, msg.Sender, msg.Executor, msg.Data, msg.DataType, msg.GasLimit).
		Suffix("ON CONFLICT (bridge_id, msg_hash) DO UPDATE SET updated_at = NOW()").
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
