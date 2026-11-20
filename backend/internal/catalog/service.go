package catalog

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"sohopos/internal/platform/apperr"
)

const (
	maxPriceCents = 10_000_000_000 // 100 million in major units
	maxStockStep  = 1_000_000
)

var skuRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,40}$`)

type Service struct {
	repo Repository
	tx   Transactor
}

func NewService(repo Repository, tx Transactor) *Service { return &Service{repo: repo, tx: tx} }

func (s *Service) ListProducts(ctx context.Context, f ProductFilter) ([]Product, error) {
	f.Search = strings.TrimSpace(f.Search)
	if r := []rune(f.Search); len(r) > 100 {
		f.Search = string(r[:100])
	}
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.ListProducts(ctx, f)
}

func (s *Service) CreateProduct(ctx context.Context, actor uuid.UUID, in CreateProductInput) (*Product, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if in.OpeningStock < 0 || in.OpeningStock > maxStockStep {
		return nil, apperr.Invalid("invalid_stock", "Starting stock must be zero or more.")
	}
	var p *Product
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		if p, err = s.repo.CreateProduct(ctx, in.ProductInput); err != nil {
			return err
		}
		if in.OpeningStock > 0 {
			p, err = s.repo.AdjustStock(ctx, p.ID, in.OpeningStock, ReasonRestock, actor)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) UpdateProduct(ctx context.Context, id uuid.UUID, in ProductInput) (*Product, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	return s.repo.UpdateProduct(ctx, id, in)
}

func (s *Service) AdjustStock(ctx context.Context, actor, id uuid.UUID, delta int, reason string) (*Product, error) {
	if delta == 0 || delta > maxStockStep || delta < -maxStockStep {
		return nil, apperr.Invalid("invalid_quantity", "Enter a whole number other than zero.")
	}
	switch reason {
	case ReasonRestock:
		if delta < 0 {
			return nil, apperr.Invalid("restock_sign", "A restock adds stock. Use a positive number.")
		}
	case ReasonDamage:
		if delta > 0 {
			return nil, apperr.Invalid("damage_sign", "Damaged stock is removed. Use a negative number.")
		}
	case ReasonCorrection:
	default:
		return nil, apperr.Invalid("invalid_reason", "Choose restock, correction or damage.")
	}
	var p *Product
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		p, err = s.repo.AdjustStock(ctx, id, delta, reason, actor)
		return err
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *Service) CreateCategory(ctx context.Context, name string) (*Category, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		return nil, apperr.Invalid("invalid_name", "Category name must be 1 to 60 characters.")
	}
	return s.repo.CreateCategory(ctx, name)
}

func (in *ProductInput) normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	in.SKU = strings.TrimSpace(in.SKU)
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 120 {
		return apperr.Invalid("invalid_name", "Name must be 1 to 120 characters.")
	}
	if !skuRe.MatchString(in.SKU) {
		return apperr.Invalid("invalid_sku", "SKU can use letters, digits, dots, dashes and underscores (up to 40).")
	}
	if in.Barcode != nil {
		b := strings.TrimSpace(*in.Barcode)
		switch {
		case b == "":
			in.Barcode = nil
		case len(b) > 64:
			return apperr.Invalid("invalid_barcode", "Barcode can be up to 64 characters.")
		default:
			in.Barcode = &b
		}
	}
	if in.PriceCents < 0 || in.PriceCents > maxPriceCents {
		return apperr.Invalid("invalid_price", "Enter a price of zero or more.")
	}
	if in.LowStockThreshold < 0 || in.LowStockThreshold > maxStockStep {
		return apperr.Invalid("invalid_threshold", "Low-stock level must be zero or more.")
	}
	return nil
}
