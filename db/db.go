package db

import (
	"amb-monitor/config"
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type DB struct {
	cfg *config.DBConfig
	*sqlx.DB
}

func (db *DB) Migrate() error {
	m, err := migrate.New("file://db/migrations", db.dbURL("pgx"))
	if err != nil {
		return err
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func (db *DB) dbURL(prefix string) string {
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s", prefix, db.cfg.User, db.cfg.Password, db.cfg.Host, db.cfg.Port, db.cfg.DB)
}

func New(cfg *config.DBConfig) (*DB, error) {
	db := &DB{
		cfg: cfg,
	}
	conn, err := sqlx.ConnectContext(context.Background(), "pgx", db.dbURL("postgres"))
	if err != nil {
		return nil, err
	}
	conn.SetMaxIdleConns(3)
	conn.SetMaxOpenConns(10)
	db.DB = conn
	return db, nil
}
