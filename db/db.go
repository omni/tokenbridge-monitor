package db

import (
	"amb-monitor/config"
	"context"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgtype/pgxtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DB struct {
	*pgxpool.Pool
}

type Executable interface {
	Exec(q pgxtype.Querier) error
	Committed()
	ApplyTo(q *DB) error
}

// ExecFuncAtomic is an atomic action, for which no transaction creation is required
type ExecFuncAtomic func(pgxtype.Querier) error
type ExecFunc func(pgxtype.Querier) error
type ExecBatch []Executable

func (f ExecFuncAtomic) Exec(q pgxtype.Querier) error {
	return f(q)
}

func (f ExecFuncAtomic) Committed() {}

func (f ExecFuncAtomic) ApplyTo(q *DB) error {
	return f.Exec(q)
}

func (f ExecFunc) Exec(q pgxtype.Querier) error {
	return f(q)
}

func (f ExecFunc) Committed() {}

func (f ExecFunc) ApplyTo(q *DB) error {
	return q.BeginFunc(context.Background(), func(tx pgx.Tx) error {
		return f.Exec(tx)
	})
}

func (fs ExecBatch) Exec(q pgxtype.Querier) error {
	for _, f := range fs {
		err := f.Exec(q)
		if err != nil {
			return err
		}
	}
	return nil
}

func (fs ExecBatch) Committed() {
	for _, f := range fs {
		f.Committed()
	}
}

func (fs ExecBatch) ApplyTo(q *DB) error {
	err := q.BeginFunc(context.Background(), func(tx pgx.Tx) error {
		return fs.Exec(tx)
	})
	if err != nil {
		return err
	}
	fs.Committed()
	return nil
}

func SanityCheck(config *config.DBConfig) error {
	migrateDbURL := fmt.Sprintf("pgx://%s:%s@%s:%d/%s", config.User, config.Password, config.Host, config.Port, config.DB)
	m, err := migrate.New("file://migrations", migrateDbURL)
	if err != nil {
		return err
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func ConnectDB(config *config.DBConfig) (*DB, error) {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", config.User, config.Password, config.Host, config.Port, config.DB)
	conn, err := pgxpool.Connect(context.Background(), dbURL)
	if err != nil {
		return nil, err
	}

	return &DB{conn}, nil
}
