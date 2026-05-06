// Package db is a thin wrapper around a pgxpool with hand-rolled query helpers
// for the small set of tables mgt-be needs. We deliberately avoid sqlc/ORMs
// to keep the dependency tree small.
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

type DB struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgx connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgx ping: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() {
	d.Pool.Close()
}

// Migrate runs every embedded migrations/*.up.sql in lexical order. It records
// applied versions in a `schema_migrations` table so re-runs are idempotent.
func (d *DB) Migrate(ctx context.Context) error {
	if _, err := d.Pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}

	entries, err := embeddedMigrations.ReadDir("migrations")
	if err != nil {
		return err
	}

	type mig struct{ version, file string }
	var ups []mig
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		version := strings.TrimSuffix(e.Name(), ".up.sql")
		ups = append(ups, mig{version: version, file: e.Name()})
	}
	sort.Slice(ups, func(i, j int) bool { return ups[i].version < ups[j].version })

	applied := map[string]bool{}
	rows, err := d.Pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	for _, m := range ups {
		if applied[m.version] {
			continue
		}
		sql, err := embeddedMigrations.ReadFile("migrations/" + m.file)
		if err != nil {
			return err
		}
		if err := withTx(ctx, d.Pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("migration %s: %w", m.version, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, m.version); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// IsNotFound reports whether err comes from a pgx Scan/QueryRow with no rows.
func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
