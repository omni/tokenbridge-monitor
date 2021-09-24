package alerts

import (
	"amb-monitor/db"
	"context"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"
)

type DBAlertsProvider struct {
	db *db.DB
}

func NewDBAlertsProvider(db *db.DB) *DBAlertsProvider {
	return &DBAlertsProvider{
		db: db,
	}
}

type UnknownConfirmation struct {
	ChainID         string         `db:"chain_id"`
	BlockNumber     uint64         `db:"block_number"`
	TransactionHash common.Hash    `db:"transaction_hash"`
	Signer          common.Address `db:"signer"`
	MsgHash         common.Hash    `db:"msg_hash"`
}

func (c *UnknownConfirmation) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"signer":       c.Signer.String(),
			"msg_hash":     c.MsgHash.String(),
		},
		Value: 1,
	}
}

func (p *DBAlertsProvider) FindUnknownConfirmations(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "sm.signer", "sm.msg_hash").
		From("signed_messages sm").
		Join("logs l ON l.id = sm.log_id").
		LeftJoin("messages m ON sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash").
		Where(sq.Eq{"m.id": nil, "sm.bridge_id": params.Bridge, "l.chain_id": params.HomeChainID}).
		Where(sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]UnknownConfirmation, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type UnknownExecution struct {
	ChainID         string      `db:"chain_id"`
	BlockNumber     uint64      `db:"block_number"`
	TransactionHash common.Hash `db:"transaction_hash"`
	MessageID       common.Hash `db:"message_id"`
}

func (c *UnknownExecution) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"message_id":   c.MessageID.String(),
		},
		Value: 1,
	}
}

func (p *DBAlertsProvider) FindUnknownExecutions(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "em.message_id").
		From("executed_messages em").
		Join("logs l ON l.id = em.log_id").
		LeftJoin("messages m ON em.bridge_id = m.bridge_id AND em.message_id = m.message_id").
		Where(sq.Eq{"m.id": nil, "em.bridge_id": params.Bridge}).
		Where(sq.Or{
			sq.And{
				sq.Eq{"l.chain_id": params.HomeChainID},
				sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber},
			},
			sq.And{
				sq.Eq{"l.chain_id": params.ForeignChainID},
				sq.GtOrEq{"l.block_number": params.ForeignStartBlockNumber},
			},
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]UnknownExecution, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type StuckMessage struct {
	ChainID         string        `db:"chain_id"`
	BlockNumber     uint64        `db:"block_number"`
	TransactionHash common.Hash   `db:"transaction_hash"`
	MsgHash         common.Hash   `db:"msg_hash"`
	Count           uint64        `db:"count"`
	WaitTime        time.Duration `db:"wait_time"`
}

func (c *StuckMessage) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"msg_hash":     c.MsgHash.String(),
			"count":        strconv.FormatUint(c.Count, 10),
		},
		Value: float64(c.WaitTime),
	}
}

func (p *DBAlertsProvider) FindStuckMessages(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	query := `
		SELECT l.chain_id,
               l.block_number,
		       l.transaction_hash,
		       sm.msg_hash,
		       count(s.log_id) as count,
	           EXTRACT(EPOCH FROM now() - ts.timestamp)::int as wait_time
		FROM sent_messages sm
		         JOIN logs l on l.id = sm.log_id
	             JOIN block_timestamps ts on ts.chain_id = l.chain_id AND ts.block_number = l.block_number
		         JOIN messages m on sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash AND data_type = 0
		         LEFT JOIN signed_messages s on s.bridge_id = m.bridge_id AND m.msg_hash = s.msg_hash
		         LEFT JOIN collected_messages cm on m.bridge_id = cm.bridge_id AND cm.msg_hash = m.msg_hash
		WHERE m.direction::text='home_to_foreign' AND cm.log_id IS NULL AND sm.bridge_id = $1 AND l.block_number >= $2 GROUP BY sm.log_id, l.id, ts.timestamp
		UNION
		SELECT l.chain_id,
               l.block_number,
		       l.transaction_hash,
		       sm.msg_hash,
		       count(s.log_id) as count,
	           EXTRACT(EPOCH FROM now() - ts.timestamp)::int as wait_time
		FROM sent_messages sm
		         JOIN logs l on l.id = sm.log_id
				 JOIN block_timestamps ts on ts.chain_id = l.chain_id AND ts.block_number = l.block_number
		         JOIN messages m on sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash AND data_type = 0
				 LEFT JOIN signed_messages s on s.bridge_id = m.bridge_id AND m.msg_hash = s.msg_hash
		         LEFT JOIN executed_messages em on m.bridge_id = em.bridge_id AND em.message_id = m.message_id
		WHERE m.direction::text='foreign_to_home' AND em.log_id IS NULL AND sm.bridge_id = $1 AND l.block_number >= $3 GROUP BY sm.log_id, l.id, ts.timestamp`
	res := make([]StuckMessage, 0, 5)
	err := p.db.SelectContext(ctx, &res, query, params.Bridge, params.HomeStartBlockNumber, params.ForeignStartBlockNumber)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type FailedExecution struct {
	ChainID         string         `db:"chain_id"`
	BlockNumber     uint64         `db:"block_number"`
	TransactionHash common.Hash    `db:"transaction_hash"`
	Sender          common.Address `db:"sender"`
	Executor        common.Address `db:"executor"`
}

func (c *FailedExecution) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id": c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":  c.TransactionHash.String(),
			"sender":   c.Sender.String(),
			"executor": c.Executor.String(),
		},
		Value: 1,
	}
}

func (p *DBAlertsProvider) FindFailedExecutions(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "m.sender", "m.executor").
		From("sent_messages sm").
		Join("messages m on sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash").
		Join("executed_messages em on m.bridge_id = em.bridge_id AND em.message_id = m.message_id").
		Join("logs l ON l.id = em.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		Where(sq.Eq{"em.status": false, "m.data_type": 0, "em.bridge_id": params.Bridge}).
		Where(sq.Or{
			sq.And{
				sq.Eq{"l.chain_id": params.HomeChainID},
				sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber},
			},
			sq.And{
				sq.Eq{"l.chain_id": params.ForeignChainID},
				sq.GtOrEq{"l.block_number": params.ForeignStartBlockNumber},
			},
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]FailedExecution, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}
