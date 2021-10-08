package postgres

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"
)

type logsRepo basePostgresRepo

func NewLogsRepo(table string, db *db.DB) entity.LogsRepo {
	return (*logsRepo)(newBasePostgresRepo(table, db))
}

func (r *logsRepo) Ensure(ctx context.Context, logs ...*entity.Log) error {
	builder := sq.Insert(r.table).
		Columns("chain_id", "address", "topic0", "topic1", "topic2", "topic3", "data", "block_number", "log_index", "transaction_hash")
	for _, log := range logs {
		builder = builder.Values(log.ChainID, log.Address, log.Topic0, log.Topic1, log.Topic2, log.Topic3, log.Data, log.BlockNumber, log.LogIndex, log.TransactionHash)
	}
	q, args, err := builder.
		Suffix("ON CONFLICT (chain_id, block_number, log_index) DO UPDATE SET updated_at = NOW()").
		Suffix("RETURNING id").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	ids := make([]uint, 0, len(logs))
	err = r.db.SelectContext(ctx, &ids, q, args...)
	if err != nil {
		return fmt.Errorf("can't get inserted log: %w", err)
	}
	if len(ids) != len(logs) {
		return fmt.Errorf("returned different number of ids then inserted, expected %d, got %d", len(logs), len(ids))
	}
	for i, id := range ids {
		logs[i].ID = id
	}
	return nil
}

func (r *logsRepo) GetByID(ctx context.Context, id uint) (*entity.Log, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	log := new(entity.Log)
	err = r.db.GetContext(ctx, log, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get log by id: %w", err)
	}
	return log, nil
}

func (r *logsRepo) FindByBlockRange(ctx context.Context, chainID string, addr common.Address, fromBlock uint, toBlock uint) ([]*entity.Log, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"chain_id": chainID, "address": addr}).
		Where(sq.LtOrEq{"block_number": toBlock}).
		Where(sq.GtOrEq{"block_number": fromBlock}).
		OrderBy("block_number", "log_index").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	logs := make([]*entity.Log, 0, 10)
	err = r.db.SelectContext(ctx, &logs, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get logs by block number: %w", err)
	}
	return logs, nil
}

func (r *logsRepo) FindByTopicAndBlockRange(ctx context.Context, chainID string, addr common.Address, fromBlock uint, toBlock uint, topic common.Hash) ([]*entity.Log, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"chain_id": chainID, "address": addr, "topic0": topic}).
		Where(sq.LtOrEq{"block_number": toBlock}).
		Where(sq.GtOrEq{"block_number": fromBlock}).
		OrderBy("block_number", "log_index").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	logs := make([]*entity.Log, 0, 10)
	err = r.db.SelectContext(ctx, &logs, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get logs by block number: %w", err)
	}
	return logs, nil
}
