package sales

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"sohopos/internal/platform/apperr"
	"sohopos/internal/platform/database"
)

type PGRepository struct{ db *database.Store }

func NewPGRepository(db *database.Store) *PGRepository { return &PGRepository{db: db} }

// ReserveStock does the check and the decrement in one UPDATE. Under READ COMMITTED a second
// checkout waits for the row lock and re-checks stock, so two cashiers can't sell the last unit.
func (r *PGRepository) ReserveStock(ctx context.Context, id uuid.UUID, qty int) (*OrderItem, error) {
	db := r.db.DB(ctx)
	it := OrderItem{ProductID: id, Quantity: qty}
	err := db.QueryRow(ctx,
		`UPDATE products SET stock_qty = stock_qty - $2, updated_at = now()
		  WHERE id = $1 AND is_active AND stock_qty >= $2
		  RETURNING name, price_cents`, id, qty).Scan(&it.ProductName, &it.UnitPriceCents)
	if err == nil {
		return &it, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// No row updated: find out why, so the cashier sees a useful message.
	var (
		name   string
		stock  int
		active bool
	)
	err = db.QueryRow(ctx, `SELECT name, stock_qty, is_active FROM products WHERE id = $1`, id).Scan(&name, &stock, &active)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, apperr.NotFound("product_not_found", "A product in this sale no longer exists.")
	case err != nil:
		return nil, err
	case !active:
		return nil, apperr.Conflict("product_unavailable", fmt.Sprintf("%s was removed from sale.", name))
	default:
		return nil, apperr.Conflict("insufficient_stock", fmt.Sprintf("Only %d of %s left in stock.", stock, name))
	}
}

func (r *PGRepository) Insert(ctx context.Context, o *Order) error {
	db := r.db.DB(ctx)
	if err := db.QueryRow(ctx,
		`INSERT INTO orders (cashier_id, total_cents, payment_method, amount_tendered_cents, change_cents)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, receipt_no, created_at`,
		o.CashierID, o.TotalCents, o.PaymentMethod, o.AmountTenderedCents, o.ChangeCents,
	).Scan(&o.ID, &o.ReceiptNo, &o.CreatedAt); err != nil {
		return err
	}

	batch := &pgx.Batch{}
	for _, it := range o.Items {
		batch.Queue(
			`INSERT INTO order_items (order_id, product_id, product_name, unit_price_cents, quantity, line_total_cents)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			o.ID, it.ProductID, it.ProductName, it.UnitPriceCents, it.Quantity, it.LineTotalCents)
		batch.Queue(
			`INSERT INTO stock_movements (product_id, delta, reason, order_id, user_id)
			 VALUES ($1, $2, 'sale', $3, $4)`,
			it.ProductID, -it.Quantity, o.ID, o.CashierID)
	}
	br := db.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return err
		}
	}
	return br.Close()
}

func (r *PGRepository) Get(ctx context.Context, id uuid.UUID, onlyCashier *uuid.UUID) (*Order, error) {
	db := r.db.DB(ctx)
	o := Order{Items: []OrderItem{}}
	// Ownership is part of the lookup itself: someone else's order is simply "not found".
	err := db.QueryRow(ctx,
		`SELECT id, receipt_no, cashier_id, total_cents, payment_method, amount_tendered_cents, change_cents, created_at
		   FROM orders
		  WHERE id = $1 AND ($2::uuid IS NULL OR cashier_id = $2::uuid)`, id, onlyCashier,
	).Scan(&o.ID, &o.ReceiptNo, &o.CashierID, &o.TotalCents, &o.PaymentMethod, &o.AmountTenderedCents, &o.ChangeCents, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("order_not_found", "Order not found.")
	}
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(ctx,
		`SELECT product_id, product_name, unit_price_cents, quantity, line_total_cents
		   FROM order_items WHERE order_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ProductID, &it.ProductName, &it.UnitPriceCents, &it.Quantity, &it.LineTotalCents); err != nil {
			return nil, err
		}
		o.Items = append(o.Items, it)
	}
	return &o, rows.Err()
}
