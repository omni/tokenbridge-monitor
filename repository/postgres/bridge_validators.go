package postgres

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"

	"github.com/omni/tokenbridge-monitor/db"
	"github.com/omni/tokenbridge-monitor/entity"
)

type bridgeValidatorsRepo basePostgresRepo

func NewBridgeValidatorsRepo(table string, db *db.DB) entity.BridgeValidatorsRepo {
	return (*bridgeValidatorsRepo)(newBasePostgresRepo(table, db))
}

func (r *bridgeValidatorsRepo) Ensure(ctx context.Context, val *entity.BridgeValidator) error {
	q, args, err := sq.Insert(r.table).
		Columns("log_id", "bridge_id", "chain_id", "address", "removed_log_id").
		Values(val.LogID, val.BridgeID, val.ChainID, val.Address, val.RemovedLogID).
		Suffix("ON CONFLICT (log_id) DO UPDATE SET updated_at = NOW(), removed_log_id = EXCLUDED.removed_log_id").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("can't build query: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("can't ensure bridge validator: %w", err)
	}
	return nil
}

func (r *bridgeValidatorsRepo) GetActiveValidator(ctx context.Context, bridgeID, chainID string, address common.Address) (*entity.BridgeValidator, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{
			"bridge_id":      bridgeID,
			"chain_id":       chainID,
			"address":        address,
			"removed_log_id": nil,
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	val := new(entity.BridgeValidator)
	err = r.db.GetContext(ctx, val, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get bridge validator: %w", err)
	}
	return val, nil
}

func (r *bridgeValidatorsRepo) FindActiveValidators(ctx context.Context, bridgeID string, chainID string) ([]*entity.BridgeValidator, error) {
	q, args, err := sq.Select("*").
		From(r.table).
		Where(sq.Eq{
			"bridge_id":      bridgeID,
			"chain_id":       chainID,
			"removed_log_id": nil,
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	vals := make([]*entity.BridgeValidator, 0, 10)
	err = r.db.SelectContext(ctx, &vals, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't get bridge validators: %w", err)
	}
	return vals, nil
}
