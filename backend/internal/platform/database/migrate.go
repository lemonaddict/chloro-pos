package database

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockID = 7270001

// Migrate applies unapplied migrations in name order, each in its own transaction.
// An advisory lock keeps two app instances from migrating at once.
//
// File names: NNNN_name.up.sql applies a migration. NNNN_name.down.sql undoes it and is never run
// here: apply it by hand (psql -f) when you really want to roll back. The older NNNN_name.sql
// form still counts as an "up" file. A migration's version is its name without .up.sql or .sql,
// so renaming 0001_schema.sql to 0001_schema.up.sql does not make it run a second time.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(migrationLockID)); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, int64(migrationLockID))
	}()

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}

	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, name := range files {
		if strings.HasSuffix(name, ".down.sql") {
			continue
		}
		version := strings.TrimSuffix(strings.TrimSuffix(name, ".sql"), ".up")
		// Rows written before the rename hold the file name (with .sql), so match those too.
		var done bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = ANY($1))`,
			[]string{version, name, version + ".sql"}).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		slog.Info("migration applied", "version", version)
	}
	return nil
}
