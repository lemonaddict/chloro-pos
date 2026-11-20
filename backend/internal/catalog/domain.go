// Package catalog owns products, categories and stock levels.
package catalog

import (
	"context"

	"github.com/google/uuid"
)

const (
	ReasonRestock    = "restock"
	ReasonCorrection = "correction"
	ReasonDamage     = "damage"
)

type Product struct {
	ID                uuid.UUID  `json:"id"`
	CategoryID        *uuid.UUID `json:"category_id"`
	CategoryName      *string    `json:"category_name"`
	SKU               string     `json:"sku"`
	Barcode           *string    `json:"barcode"`
	Name              string     `json:"name"`
	PriceCents        int64      `json:"price_cents"`
	StockQty          int        `json:"stock_qty"`
	LowStockThreshold int        `json:"low_stock_threshold"`
	IsActive          bool       `json:"is_active"`
}

type Category struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ProductInput is the allowlist of fields a client may write. Stock is deliberately absent:
// it changes only through audited stock adjustments and through sales.
type ProductInput struct {
	SKU               string     `json:"sku"`
	Barcode           *string    `json:"barcode"`
	Name              string     `json:"name"`
	CategoryID        *uuid.UUID `json:"category_id"`
	PriceCents        int64      `json:"price_cents"`
	LowStockThreshold int        `json:"low_stock_threshold"`
	IsActive          *bool      `json:"is_active"` // nil: active on create, unchanged on update
}

type CreateProductInput struct {
	ProductInput
	OpeningStock int `json:"opening_stock"`
}

type ProductFilter struct {
	Search          string
	CategoryID      *uuid.UUID
	IncludeInactive bool
	LowStock        bool // only products at or below their own low-stock level, lowest first
	Limit, Offset   int
}

// Repository is the persistence port. The Postgres adapter lives in repository_pg.go.
type Repository interface {
	ListProducts(ctx context.Context, f ProductFilter) ([]Product, error)
	GetProduct(ctx context.Context, id uuid.UUID) (*Product, error)
	CreateProduct(ctx context.Context, in ProductInput) (*Product, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, in ProductInput) (*Product, error)
	AdjustStock(ctx context.Context, id uuid.UUID, delta int, reason string, actor uuid.UUID) (*Product, error)
	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, name string) (*Category, error)
}

// Transactor lets a use case group repository calls into one transaction.
type Transactor interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
