package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
)

type blockTimestampsRepo basePostgresRepo

func NewBlockTimestampsRepo(table string, db *db.DB) entity.BlockTimestampsRepo {
	return (*blockTimestampsRepo)(newBasePostgresRepo(table, db))
}

func (r *blockTimestampsRepo) Ensure(ctx context.Context, ts *entity.BlockTimestamp) error {
	q, args, err := sq.Insert(r.table).
		Columns("chain_id", "block_number", "timestamp").
		Values(ts.ChainID, ts.BlockNumber, ts.Timestamp).
		Suffix("ON CONFLICT (chain_id, block_number) DO UPDATE SET updated_at = NOW(), timestamp = EXCLUDED.timestamp").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert block timestamp: %w", err)
	}
	return nil
}

func (r *blockTimestampsRepo) GetByBlockNumber(ctx context.Context, chainID string, blockNumber uint) (*entity.BlockTimestamp, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{
			"chain_id":     chainID,
			"block_number": blockNumber,
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	bt := new(entity.BlockTimestamp)
	err = r.db.GetContext(ctx, bt, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get block timestamp: %w", err)
	}
	return bt, nil
}
