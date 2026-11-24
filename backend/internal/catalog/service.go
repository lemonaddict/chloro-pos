package catalog

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/database"
)

const (
	maxPriceCents = 10_000_000_000 // Rp 100.000.000 in minor units
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
	return database.InTx(ctx, s.tx, func(ctx context.Context) ([]Product, error) {
		return s.repo.ListProducts(ctx, f)
	})
}

func (s *Service) CreateProduct(ctx context.Context, actor uuid.UUID, in CreateProductInput) (*Product, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if in.OpeningStock < 0 || in.OpeningStock > maxStockStep {
		return nil, apperr.Invalid("invalid_stock", "Stok awal harus nol atau lebih.")
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
	return database.InTx(ctx, s.tx, func(ctx context.Context) (*Product, error) {
		return s.repo.UpdateProduct(ctx, id, in)
	})
}

func (s *Service) AdjustStock(ctx context.Context, actor, id uuid.UUID, delta int, reason string) (*Product, error) {
	if delta == 0 || delta > maxStockStep || delta < -maxStockStep {
		return nil, apperr.Invalid("invalid_quantity", "Isi dengan bilangan bulat selain nol.")
	}
	switch reason {
	case ReasonRestock:
		if delta < 0 {
			return nil, apperr.Invalid("restock_sign", "Restock menambah stok. Pakai angka positif.")
		}
	case ReasonDamage:
		if delta > 0 {
			return nil, apperr.Invalid("damage_sign", "Barang rusak atau hilang mengurangi stok. Pakai angka negatif.")
		}
	case ReasonCorrection:
	default:
		return nil, apperr.Invalid("invalid_reason", "Pilih restock, koreksi, atau rusak/hilang.")
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
	return database.InTx(ctx, s.tx, s.repo.ListCategories)
}

func (s *Service) CreateCategory(ctx context.Context, name string) (*Category, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		return nil, apperr.Invalid("invalid_name", "Nama kategori harus 1 sampai 60 karakter.")
	}
	return database.InTx(ctx, s.tx, func(ctx context.Context) (*Category, error) {
		return s.repo.CreateCategory(ctx, name)
	})
}

func (in *ProductInput) normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	in.SKU = strings.TrimSpace(in.SKU)
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 120 {
		return apperr.Invalid("invalid_name", "Nama produk harus 1 sampai 120 karakter.")
	}
	if !skuRe.MatchString(in.SKU) {
		return apperr.Invalid("invalid_sku", "SKU boleh berisi huruf, angka, titik, strip, dan garis bawah (maksimal 40).")
	}
	if in.Barcode != nil {
		b := strings.TrimSpace(*in.Barcode)
		switch {
		case b == "":
			in.Barcode = nil
		case len(b) > 64:
			return apperr.Invalid("invalid_barcode", "Barcode maksimal 64 karakter.")
		default:
			in.Barcode = &b
		}
	}
	if in.PriceCents < 0 || in.PriceCents > maxPriceCents {
		return apperr.Invalid("invalid_price", "Isi harga nol atau lebih, maksimal Rp 100.000.000.")
	}
	if in.PriceCents%100 != 0 { // minor units are x100; rupiah has no sen in practice
		return apperr.Invalid("invalid_price", "Harga harus dalam rupiah penuh, tanpa sen.")
	}
	if in.LowStockThreshold < 0 || in.LowStockThreshold > maxStockStep {
		return apperr.Invalid("invalid_threshold", "Batas stok menipis harus nol atau lebih.")
	}
	return nil
}
