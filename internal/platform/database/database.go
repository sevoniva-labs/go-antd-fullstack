package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sevoniva-labs/forge/internal/platform/config"
)

type DB struct {
	*sql.DB
	Provider string
}

func Open(ctx context.Context, cfg config.Database) (*DB, error) {
	driver := "pgx"
	if cfg.Provider == "mysql" || cfg.Provider == "oceanbase" {
		driver = "mysql"
	}
	raw, err := sql.Open(driver, cfg.DSN)
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(cfg.MaxOpenConns)
	raw.SetMaxIdleConns(cfg.MaxIdleConns)
	raw.SetConnMaxLifetime(cfg.MaxLifetime)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := raw.PingContext(pingCtx); err != nil {
		raw.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}
	return &DB{DB: raw, Provider: cfg.Provider}, nil
}

func (db *DB) Rebind(query string) string {
	if db.Provider != "postgres" {
		return query
	}
	var b strings.Builder
	n := 1
	for _, r := range query {
		if r == '?' {
			fmt.Fprintf(&b, "$%d", n)
			n++
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (db *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
