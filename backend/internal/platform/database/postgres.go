// Package database owns the connection pool, transactions and migrations.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chloropos/internal/platform/tenancy"
)

// NewPool builds a bounded pool with server-side guard rails (statement and idle-in-transaction timeouts).
func NewPool(ctx context.Context, url string, maxConns, minConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnLifetimeJitter = 5 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = time.Minute
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.RuntimeParams["application_name"] = "chloropos"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "15000"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "30000"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("reach database: %w", err)
	}
	return pool, nil
}

// DBTX is what repositories talk to. It is satisfied by both the pool and an open transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type txKey struct{}

// Store hands repositories the right DBTX for the current context, so a use case can wrap
// several repository calls in one transaction without repositories knowing about it.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) DB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return s.pool
}

// WithTx runs fn in a transaction (READ COMMITTED). Nested calls join the outer transaction.
//
// When the context carries a tenant, the transaction is pinned to it: row-level security then
// shows only that tenant's rows and rejects writes for any other. Tables that hold a tenant's
// data are unreadable outside a transaction, so a use case that forgets WithTx gets no rows
// instead of someone else's.
func (s *Store) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // no-op after a successful commit
	if tid, ok := tenancy.From(ctx); ok {
		// is_local = true: the setting ends with the transaction, so a pooled connection never carries it over.
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tid.String()); err != nil {
			return err
		}
	}
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// BypassesRLS reports whether the connected role ignores row-level security (a superuser, or a
// role with BYPASSRLS). The policies then protect nothing, and tenant isolation rests on the
// tenant filter in every repository query alone.
func (s *Store) BypassesRLS(ctx context.Context) (bool, error) {
	var bypass bool
	err := s.pool.QueryRow(ctx, `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypass)
	return bypass, err
}

// Transactor is what use cases need from the Store.
type Transactor interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// InTx runs fn in a transaction and hands back what it returns. Read use cases use it so that
// their queries run inside the tenant-pinned transaction too.
func InTx[T any](ctx context.Context, tx Transactor, fn func(ctx context.Context) (T, error)) (T, error) {
	var out T
	err := tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		out, err = fn(ctx)
		return err
	})
	return out, err
}

// UniqueViolation returns the violated constraint or index name for SQLSTATE 23505.
func UniqueViolation(err error) (string, bool) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return pe.ConstraintName, true
	}
	return "", false
}

// IsForeignKeyViolation reports SQLSTATE 23503.
func IsForeignKeyViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23503"
}
