// Package sales owns checkout, tax calculation and receipts.
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
	ReceiptNo           int64       `json:"receipt_no"` // counts per shop, from 1
	CashierName         string      `json:"cashier_name"` // who rang it up, as named when the sale was made
	Items               []OrderItem `json:"items"`
	SubtotalCents       int64       `json:"subtotal_cents"`
	TaxCents            int64       `json:"tax_cents"`
	TaxRateBPS          int         `json:"tax_rate_bps"`
	TaxInclusive        bool        `json:"tax_inclusive"`
	TaxLabel            string      `json:"tax_label"`
	TotalCents          int64       `json:"total_cents"`
	PaymentMethod       string      `json:"payment_method"`
	AmountTenderedCents int64       `json:"amount_tendered_cents"`
	ChangeCents         int64       `json:"change_cents"`
	CreatedAt           time.Time   `json:"created_at"`

	// SeatID is the kasir seat that made the sale; nil means the owner sold it at the till.
	// It stays on the server: the browser only needs the name.
	SeatID *uuid.UUID `json:"-"`
}

// TaxConfig is the tax rule in force at checkout. It is copied onto the order.
type TaxConfig struct {
	Enabled   bool
	Label     string
	RateBPS   int  // 1100 = 11%
	Inclusive bool // prices already include tax
}

// TaxSource is implemented by the settings module and wired in internal/app.
type TaxSource interface {
	Tax(ctx context.Context) (TaxConfig, error)
}

// Repository is the persistence port. The shop (tenant) always comes from the context.
type Repository interface {
	// ReserveStock atomically takes qty units and returns the line priced from the database.
	ReserveStock(ctx context.Context, productID uuid.UUID, qty int) (*OrderItem, error)
	// Insert stores the order, its lines and the matching stock movements, and numbers the receipt.
	Insert(ctx context.Context, o *Order) error
	// Get loads an order. When onlySeat is set the lookup is limited to that kasir seat's orders.
	Get(ctx context.Context, id uuid.UUID, onlySeat *uuid.UUID) (*Order, error)
}

type Transactor interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
