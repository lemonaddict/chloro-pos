// Package sales owns checkout and receipts.
package sales

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type OrderItem struct {
	ProductID      uuid.UUID `json:"product_id"`
	ProductName    string    `json:"product_name"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	Quantity       int       `json:"quantity"`
	LineTotalCents int64     `json:"line_total_cents"`
}

type Order struct {
	ID                  uuid.UUID   `json:"id"`
	ReceiptNo           int64       `json:"receipt_no"`
	CashierID           uuid.UUID   `json:"cashier_id"`
	Items               []OrderItem `json:"items"`
	TotalCents          int64       `json:"total_cents"`
	PaymentMethod       string      `json:"payment_method"`
	AmountTenderedCents int64       `json:"amount_tendered_cents"`
	ChangeCents         int64       `json:"change_cents"`
	CreatedAt           time.Time   `json:"created_at"`
}

type Repository interface {
	// ReserveStock atomically takes qty units and returns the line priced from the database.
	ReserveStock(ctx context.Context, productID uuid.UUID, qty int) (*OrderItem, error)
	// Insert stores the order, its lines and the matching stock movements.
	Insert(ctx context.Context, o *Order) error
	// Get loads an order. When onlyCashier is set the lookup is limited to that cashier's orders.
	Get(ctx context.Context, id uuid.UUID, onlyCashier *uuid.UUID) (*Order, error)
}

type Transactor interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
