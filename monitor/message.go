package monitor

import (
	"context"
	"github.com/jackc/pgtype/pgxtype"
	"github.com/jackc/pgx/v4"
)

const (
	HomeToForeign = iota
	ForeignToHome
)

type Message struct {
	dbId      int
	BridgeId  string
	Direction int
	MsgHash   string
	MessageId string
	Sender    string
	Executor  string
	GasLimit  uint32
	DataType  uint8
	Data      string
}

type TxLogInfo struct {
	ChainId     string
	TxHash      string
	BlockNumber uint64
	LogIndex    uint
}

type MessageRequest struct {
	Message *Message
	*TxLogInfo
}

type MessageConfirmation struct {
	Message   *Message
	Validator string
	// TODO Signature
	*TxLogInfo
}

type MessageExecution struct {
	Message *Message
	Status  bool
	*TxLogInfo
}

type Insertable interface {
	Insert(pgxtype.Querier) error
}

func (m *Message) Insert(q pgxtype.Querier) error {
	return q.QueryRow(
		context.Background(),
		"INSERT INTO message "+
			"(msg_hash, bridge_id, direction, message_id, sender, executor, gas_limit, data_type, data) "+
			"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) "+
			"ON CONFLICT (bridge_id, message_id, msg_hash) DO NOTHING "+
			"RETURNING id",
		m.MsgHash, m.BridgeId, m.Direction == ForeignToHome, m.MessageId, m.Sender, m.Executor, m.GasLimit, m.DataType, m.Data,
	).Scan(&m.dbId)
}

func (m *MessageRequest) Insert(q pgxtype.Querier) error {
	if m.Message.dbId == 0 {
		return nil
	}
	_, err := q.Exec(
		context.Background(),
		"INSERT INTO message_request (msg_id, chain_id, tx_hash, block_number, log_index) "+
			"VALUES ($1, $2, $3, $4, $5) "+
			"ON CONFLICT (msg_id) DO NOTHING",
		m.Message.dbId,
		m.ChainId, m.TxHash, m.BlockNumber, m.LogIndex,
	)
	return err
}

func (m *MessageConfirmation) Insert(q pgxtype.Querier) error {
	err := q.QueryRow(
		context.Background(),
		"SELECT id FROM message WHERE bridge_id = $1 AND msg_hash = $2",
		m.Message.BridgeId, m.Message.MsgHash,
	).Scan(&m.Message.dbId)
	if err == pgx.ErrNoRows {
		_, err = q.Exec(
			context.Background(),
			"INSERT INTO message_confirmation (msg_id, validator, chain_id, tx_hash, block_number, log_index, tmp_bridge_id, tmp_msg_hash) "+
				"VALUES (NULL, $1, $2, $3, $4, $5, $6, $7) "+
				"ON CONFLICT (tx_hash, validator) DO NOTHING "+
				"RETURNING id ",
			m.Validator,
			m.ChainId, m.TxHash, m.BlockNumber, m.LogIndex,
			m.Message.BridgeId, m.Message.MsgHash,
		)
		return err
	}
	if err != nil {
		return err
	}
	_, err = q.Exec(
		context.Background(),
		"INSERT INTO message_confirmation (msg_id, validator, chain_id, tx_hash, block_number, log_index) "+
			"VALUES ($1, $2, $3, $4, $5, $6) "+
			"ON CONFLICT (tx_hash, validator) DO NOTHING "+
			"RETURNING id",
		m.Message.dbId,
		m.Validator,
		m.ChainId, m.TxHash, m.BlockNumber, m.LogIndex,
	)
	return err
}

func (m *MessageExecution) Insert(q pgxtype.Querier) error {
	err := q.QueryRow(
		context.Background(),
		"SELECT id FROM message WHERE bridge_id = $1 AND message_id = $2",
		m.Message.BridgeId, m.Message.MessageId,
	).Scan(&m.Message.dbId)
	if err == pgx.ErrNoRows {
		_, err = q.Exec(
			context.Background(),
			"INSERT INTO message_execution (msg_id, status, chain_id, tx_hash, block_number, log_index, tmp_bridge_id, tmp_message_id) "+
				"VALUES (NULL, $1, $2, $3, $4, $5, $6, $7) "+
				"ON CONFLICT (msg_id) DO NOTHING "+
				"RETURNING id",
			m.Status,
			m.ChainId, m.TxHash, m.BlockNumber, m.LogIndex,
			m.Message.BridgeId, m.Message.MessageId,
		)
		return err
	}
	if err != nil {
		return err
	}
	_, err = q.Exec(
		context.Background(),
		"INSERT INTO message_execution (msg_id, status, chain_id, tx_hash, block_number, log_index) "+
			"VALUES ($1, $2, $3, $4, $5, $6) "+
			"ON CONFLICT (msg_id) DO NOTHING "+
			"RETURNING id",
		m.Message.dbId,
		m.Status,
		m.ChainId, m.TxHash, m.BlockNumber, m.LogIndex,
	)
	return err
}
