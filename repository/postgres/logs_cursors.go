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

type logsCursorsRepo basePostgresRepo

func NewLogsCursorRepo(table string, db *db.DB) entity.LogsCursorsRepo {
	return (*logsCursorsRepo)(newBasePostgresRepo(table, db))
}

func (r *logsCursorsRepo) Ensure(ctx context.Context, cursor *entity.LogsCursor) error {
	q, args, err := sq.Insert(r.table).
		Columns("chain_id", "address", "last_fetched_block", "last_processed_block").
		Values(cursor.ChainID, cursor.Address, cursor.LastFetchedBlock, cursor.LastProcessedBlock).
		Suffix("ON CONFLICT (chain_id, address) DO UPDATE SET updated_at = NOW(), last_fetched_block = EXCLUDED.last_fetched_block, last_processed_block = EXCLUDED.last_processed_block").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't insert logs cursor: %w", err)
	}
	return nil
}

func (r *logsCursorsRepo) GetByChainIDAndAddress(ctx context.Context, chainID string, addr common.Address) (*entity.LogsCursor, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{"chain_id": chainID, "address": addr}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	log := new(entity.LogsCursor)
	err = r.db.GetContext(ctx, log, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, fmt.Errorf("can't get logs cursor by chain_id and address: %w", err)
	}
	return log, nil
}
