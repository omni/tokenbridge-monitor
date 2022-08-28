package postgres

import (
	"github.com/omni/tokenbridge-monitor/db"
)

type basePostgresRepo struct {
	table string
	db    *db.DB
}

func newBasePostgresRepo(table string, db *db.DB) *basePostgresRepo {
	return &basePostgresRepo{
		table: table,
		db:    db,
	}
}
