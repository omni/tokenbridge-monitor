package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"

	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
)

type messagesRepo basePostgresRepo

func NewMessagesRepo(table string, db *db.DB) entity.MessagesRepo {
	return (*messagesRepo)(newBasePostgresRepo(table, db))
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

func (r *messagesRepo) GetByMsgHash(ctx context.Context, bridgeID string, msgHash common.Hash) (*entity.Message, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "msg_hash": msgHash}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.Message)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get message: %w", err)
	}
	return msg, nil
}

func (r *messagesRepo) GetByMessageID(ctx context.Context, bridgeID string, messageID common.Hash) (*entity.Message, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"bridge_id": bridgeID, "message_id": messageID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msg := new(entity.Message)
	err = r.db.GetContext(ctx, msg, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get message: %w", err)
	}
	return msg, nil
}

func (r *messagesRepo) FindPendingMessages(ctx context.Context, bridgeID string) ([]*entity.Message, error) {
	q, args, err := sq.Select("m.*").
		From(r.table + " m").
		LeftJoin("executed_messages em ON em.message_id = m.message_id AND em.bridge_id = m.bridge_id").
		Where(sq.Eq{"m.bridge_id": bridgeID, "em.log_id": nil}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	msgs := make([]*entity.Message, 0, 10)
	err = r.db.SelectContext(ctx, &msgs, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't find messages: %w", err)
	}
	return msgs, nil
}
