package postgres

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
)

type logsRepo basePostgresRepo

var ErrInvalidPostgresResult = errors.New("postgres returned invalid result")

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
		return fmt.Errorf("returned different number of ids then inserted, expected %d, got %d: %w", len(logs), len(ids), ErrInvalidPostgresResult)
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

//nolint:cyclop
func (r *logsRepo) Find(ctx context.Context, filter entity.LogsFilter) ([]*entity.Log, error) {
	cond := sq.And{}
	if filter.ChainID != nil {
		cond = append(cond, sq.Eq{"chain_id": *filter.ChainID})
	}
	if len(filter.Addresses) > 0 {
		cond = append(cond, sq.Eq{"address": filter.Addresses})
	}
	if filter.FromBlock != nil {
		cond = append(cond, sq.GtOrEq{"block_number": *filter.FromBlock})
	}
	if filter.ToBlock != nil {
		cond = append(cond, sq.LtOrEq{"block_number": *filter.ToBlock})
	}
	if filter.TxHash != nil {
		cond = append(cond, sq.Eq{"transaction_hash": *filter.TxHash})
	}
	if len(filter.Topic0) > 0 {
		cond = append(cond, sq.Eq{"topic0": filter.Topic0})
	}
	if len(filter.Topic1) > 0 {
		cond = append(cond, sq.Eq{"topic1": filter.Topic1})
	}
	if len(filter.Topic2) > 0 {
		cond = append(cond, sq.Eq{"topic2": filter.Topic2})
	}
	if len(filter.Topic3) > 0 {
		cond = append(cond, sq.Eq{"topic3": filter.Topic3})
	}
	if filter.DataLength != nil {
		cond = append(cond, sq.Eq{"length(data)": *filter.DataLength})
	}

	q, args, err := sq.Select("*").
		From(r.table).
		Where(cond).
		OrderBy("chain_id", "block_number", "log_index").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	logs := make([]*entity.Log, 0, 10)
	err = r.db.SelectContext(ctx, &logs, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't find logs by filter query: %w", err)
	}
	return logs, nil
}
