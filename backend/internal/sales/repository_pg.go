package sales

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/database"
	"chloropos/internal/platform/tenancy"
)

// Every query filters by the tenant from the context, on top of the row-level security the
// database applies, so a shop only ever sells from and reads its own data.

type PGRepository struct{ db *database.Store }

func NewPGRepository(db *database.Store) *PGRepository { return &PGRepository{db: db} }

// ReserveStock does the check and the decrement in one UPDATE. Under READ COMMITTED a second
// checkout waits for the row lock and re-checks stock, so two cashiers can't sell the last unit.
func (r *PGRepository) ReserveStock(ctx context.Context, id uuid.UUID, qty int) (*OrderItem, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	db := r.db.DB(ctx)
	it := OrderItem{ProductID: id, Quantity: qty}
	err = db.QueryRow(ctx,
		`UPDATE products SET stock_qty = stock_qty - $2, updated_at = now()
		  WHERE id = $1 AND tenant_id = $3 AND is_active AND stock_qty >= $2
		  RETURNING name, price_cents`, id, qty, tid).Scan(&it.ProductName, &it.UnitPriceCents)
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
	err = db.QueryRow(ctx, `SELECT name, stock_qty, is_active FROM products WHERE id = $1 AND tenant_id = $2`, id, tid).Scan(&name, &stock, &active)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, apperr.NotFound("product_not_found", "Ada produk di penjualan ini yang sudah dihapus.")
	case err != nil:
		return nil, err
	case !active:
		return nil, apperr.Conflict("product_unavailable", fmt.Sprintf("%s sudah tidak dijual.", name))
	case stock <= 0:
		return nil, apperr.Conflict("insufficient_stock", fmt.Sprintf("Stok %s habis.", name))
	default:
		return nil, apperr.Conflict("insufficient_stock", fmt.Sprintf("Stok %s hanya tersisa %d.", name, stock))
	}
}

func (r *PGRepository) Insert(ctx context.Context, o *Order) error {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return err
	}
	db := r.db.DB(ctx)

	// Receipt numbers count per shop, with no gaps: the counter row stays locked until this
	// transaction ends, and a rolled-back sale gives its number back. The upsert also covers a
	// shop that has not opened its settings yet.
	var receiptNo int64
	if err := db.QueryRow(ctx,
		`INSERT INTO settings (tenant_id, business_name, last_receipt_no)
		 SELECT id, display_name, 1 FROM users WHERE id = $1
		 ON CONFLICT (tenant_id) DO UPDATE SET last_receipt_no = settings.last_receipt_no + 1
		 RETURNING last_receipt_no`, tid).Scan(&receiptNo); err != nil {
		return err
	}

	// sold_by_name is copied onto the order, so renaming a kasir never rewrites an old receipt.
	// seat_id nil means the owner sold it. A seat of another shop fails the composite foreign key.
	if err := db.QueryRow(ctx,
		`INSERT INTO orders (tenant_id, receipt_no, seat_id, sold_by_name, subtotal_cents, tax_cents, tax_rate_bps,
		                     tax_inclusive, tax_label, total_cents, payment_method, amount_tendered_cents, change_cents)
		 VALUES ($1::uuid, $2, $3::uuid,
		         CASE WHEN $3::uuid IS NULL THEN (SELECT display_name FROM users WHERE id = $1::uuid)
		              ELSE (SELECT name FROM cashier_seats WHERE id = $3::uuid AND tenant_id = $1::uuid) END,
		         $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, created_at, sold_by_name`,
		tid, receiptNo, o.SeatID, o.SubtotalCents, o.TaxCents, o.TaxRateBPS, o.TaxInclusive, o.TaxLabel,
		o.TotalCents, o.PaymentMethod, o.AmountTenderedCents, o.ChangeCents,
	).Scan(&o.ID, &o.CreatedAt, &o.CashierName); err != nil {
		return err
	}
	o.ReceiptNo = receiptNo

	batch := &pgx.Batch{}
	for _, it := range o.Items {
		batch.Queue(
			`INSERT INTO order_items (tenant_id, order_id, product_id, product_name, unit_price_cents, quantity, line_total_cents)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			tid, o.ID, it.ProductID, it.ProductName, it.UnitPriceCents, it.Quantity, it.LineTotalCents)
		// The movement is credited to the seat, or to the owner when the owner sold it.
		batch.Queue(
			`INSERT INTO stock_movements (tenant_id, product_id, delta, reason, order_id, user_id, seat_id)
			 VALUES ($1::uuid, $2, $3, 'sale', $4, CASE WHEN $5::uuid IS NULL THEN $1::uuid END, $5::uuid)`,
			tid, it.ProductID, -it.Quantity, o.ID, o.SeatID)
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

func (r *PGRepository) Get(ctx context.Context, id uuid.UUID, onlySeat *uuid.UUID) (*Order, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	db := r.db.DB(ctx)
	o := Order{Items: []OrderItem{}}
	// Ownership is part of the lookup itself: another shop's order, or another seat's, is simply "not found".
	err = db.QueryRow(ctx,
		`SELECT o.id, o.receipt_no, o.sold_by_name, o.subtotal_cents, o.tax_cents, o.tax_rate_bps,
		        o.tax_inclusive, o.tax_label, o.total_cents, o.payment_method, o.amount_tendered_cents,
		        o.change_cents, o.created_at
		   FROM orders o
		  WHERE o.id = $1 AND o.tenant_id = $3 AND ($2::uuid IS NULL OR o.seat_id = $2::uuid)`, id, onlySeat, tid,
	).Scan(&o.ID, &o.ReceiptNo, &o.CashierName, &o.SubtotalCents, &o.TaxCents, &o.TaxRateBPS,
		&o.TaxInclusive, &o.TaxLabel, &o.TotalCents, &o.PaymentMethod, &o.AmountTenderedCents,
		&o.ChangeCents, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("order_not_found", "Transaksi tidak ditemukan.")
	}
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(ctx,
		`SELECT product_id, product_name, unit_price_cents, quantity, line_total_cents
		   FROM order_items WHERE order_id = $1 AND tenant_id = $2 ORDER BY id`, id, tid)
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
