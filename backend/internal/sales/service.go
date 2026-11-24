package sales

import (
	"bytes"
	"context"
	"sort"

	"github.com/google/uuid"

	"chloropos/internal/platform/apperr"
)

const (
	maxLines         = 100
	maxQtyPerProduct = 9999
	maxTenderedCents = 1_000_000_000_000_000
)

var paymentMethods = map[string]bool{"cash": true, "card": true, "transfer": true, "qris": true}

type LineInput struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int       `json:"quantity"`
}

// CheckoutInput carries no prices and no tax on purpose: the server sets both.
type CheckoutInput struct {
	Items               []LineInput `json:"items"`
	PaymentMethod       string      `json:"payment_method"`
	AmountTenderedCents int64       `json:"amount_tendered_cents"`
}

type Service struct {
	repo Repository
	tx   Transactor
	tax  TaxSource
}

func NewService(repo Repository, tx Transactor, tax TaxSource) *Service {
	return &Service{repo: repo, tx: tx, tax: tax}
}

// Checkout is one transaction: reserve stock, price the lines, add tax, check payment, store the order.
// If any step fails, nothing is kept. seatID is the kasir seat making the sale, or nil when the owner sells.
func (s *Service) Checkout(ctx context.Context, seatID *uuid.UUID, in CheckoutInput) (*Order, error) {
	lines, err := mergeLines(in.Items)
	if err != nil {
		return nil, err
	}
	if !paymentMethods[in.PaymentMethod] {
		return nil, apperr.Invalid("bad_payment_method", "Pilih tunai, QRIS, transfer, atau kartu.")
	}
	if in.AmountTenderedCents < 0 || in.AmountTenderedCents > maxTenderedCents {
		return nil, apperr.Invalid("bad_amount", "Isi jumlah uang yang diterima.")
	}
	taxCfg, err := s.tax.Tax(ctx)
	if err != nil {
		return nil, err
	}

	var order *Order
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		items := make([]OrderItem, 0, len(lines))
		var subtotal int64
		for _, l := range lines {
			it, err := s.repo.ReserveStock(ctx, l.ProductID, l.Quantity)
			if err != nil {
				return err
			}
			it.LineTotalCents = it.UnitPriceCents * int64(it.Quantity)
			subtotal += it.LineTotalCents
			items = append(items, *it)
		}

		tax, total := ComputeTax(subtotal, taxCfg)
		tendered := in.AmountTenderedCents
		if in.PaymentMethod != "cash" {
			tendered = total // QRIS, transfer and card are taken at the exact amount
		}
		if tendered < total {
			return apperr.Invalid("insufficient_payment", "Uang yang diterima kurang dari total.")
		}

		order = &Order{
			SeatID:              seatID,
			Items:               items,
			SubtotalCents:       subtotal,
			TaxCents:            tax,
			TotalCents:          total,
			PaymentMethod:       in.PaymentMethod,
			AmountTenderedCents: tendered,
			ChangeCents:         tendered - total,
		}
		if taxCfg.Enabled && taxCfg.RateBPS > 0 {
			order.TaxRateBPS = taxCfg.RateBPS
			order.TaxInclusive = taxCfg.Inclusive
			order.TaxLabel = taxCfg.Label
		}
		return s.repo.Insert(ctx, order)
	})
	if err != nil {
		return nil, err
	}
	return order, nil
}

// ComputeTax returns the tax amount and the total to pay, in minor units (x100). Rupiah has no
// sen in practice, so tax is rounded to a whole rupiah.
//
//	Exclusive: prices exclude tax. total = subtotal + tax.
//	Inclusive: prices already include tax. total = subtotal, and tax is the part inside it.
func ComputeTax(subtotal int64, t TaxConfig) (tax, total int64) {
	if !t.Enabled || t.RateBPS <= 0 {
		return 0, subtotal
	}
	rate := int64(t.RateBPS)
	rupiah := subtotal / 100
	if t.Inclusive {
		den := 10000 + rate
		return ((rupiah*rate + den/2) / den) * 100, subtotal
	}
	tax = ((rupiah*rate + 5000) / 10000) * 100
	return tax, subtotal + tax
}

// GetOrder lets an owner open any order of their shop, and a kasir seat only its own.
func (s *Service) GetOrder(ctx context.Context, id uuid.UUID, onlySeat *uuid.UUID) (*Order, error) {
	var o *Order
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		o, err = s.repo.Get(ctx, id, onlySeat)
		return err
	})
	return o, err
}

// mergeLines validates the cart, folds duplicate products together and sorts by product ID.
// A fixed lock order stops two checkouts that share products from deadlocking each other.
func mergeLines(in []LineInput) ([]LineInput, error) {
	if len(in) == 0 {
		return nil, apperr.Invalid("empty_cart", "Tambahkan minimal satu produk.")
	}
	if len(in) > maxLines {
		return nil, apperr.Invalid("too_many_lines", "Satu penjualan maksimal 100 jenis produk.")
	}
	qty := make(map[uuid.UUID]int, len(in))
	for _, l := range in {
		if l.Quantity < 1 {
			return nil, apperr.Invalid("bad_quantity", "Jumlah minimal 1.")
		}
		qty[l.ProductID] += l.Quantity
		if qty[l.ProductID] > maxQtyPerProduct {
			return nil, apperr.Invalid("bad_quantity", "Jumlah maksimal 9.999 per produk.")
		}
	}
	out := make([]LineInput, 0, len(qty))
	for id, q := range qty {
		out = append(out, LineInput{ProductID: id, Quantity: q})
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i].ProductID[:], out[j].ProductID[:]) < 0 })
	return out, nil
}
