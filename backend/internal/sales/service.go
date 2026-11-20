package sales

import (
	"bytes"
	"context"
	"sort"

	"github.com/google/uuid"

	"sohopos/internal/platform/apperr"
)

const (
	maxLines         = 100
	maxQtyPerProduct = 9999
	maxTenderedCents = 1_000_000_000_000_000
)

var paymentMethods = map[string]bool{"cash": true, "card": true, "transfer": true}

type LineInput struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int       `json:"quantity"`
}

// CheckoutInput carries no prices on purpose: the server prices every line from the database.
type CheckoutInput struct {
	Items               []LineInput `json:"items"`
	PaymentMethod       string      `json:"payment_method"`
	AmountTenderedCents int64       `json:"amount_tendered_cents"`
}

type Service struct {
	repo Repository
	tx   Transactor
}

func NewService(repo Repository, tx Transactor) *Service { return &Service{repo: repo, tx: tx} }

// Checkout is one transaction: reserve stock, price the lines, check payment, store the order.
// If any step fails, nothing is kept.
func (s *Service) Checkout(ctx context.Context, cashierID uuid.UUID, in CheckoutInput) (*Order, error) {
	lines, err := mergeLines(in.Items)
	if err != nil {
		return nil, err
	}
	if !paymentMethods[in.PaymentMethod] {
		return nil, apperr.Invalid("bad_payment_method", "Choose cash, card or transfer.")
	}
	if in.AmountTenderedCents < 0 || in.AmountTenderedCents > maxTenderedCents {
		return nil, apperr.Invalid("bad_amount", "Enter the amount received.")
	}

	var order *Order
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		items := make([]OrderItem, 0, len(lines))
		var total int64
		for _, l := range lines {
			it, err := s.repo.ReserveStock(ctx, l.ProductID, l.Quantity)
			if err != nil {
				return err
			}
			it.LineTotalCents = it.UnitPriceCents * int64(it.Quantity)
			total += it.LineTotalCents
			items = append(items, *it)
		}

		tendered := in.AmountTenderedCents
		if in.PaymentMethod != "cash" {
			tendered = total // card and transfer are taken at the exact amount
		}
		if tendered < total {
			return apperr.Invalid("insufficient_payment", "The amount received is less than the total.")
		}

		order = &Order{
			CashierID:           cashierID,
			Items:               items,
			TotalCents:          total,
			PaymentMethod:       in.PaymentMethod,
			AmountTenderedCents: tendered,
			ChangeCents:         tendered - total,
		}
		return s.repo.Insert(ctx, order)
	})
	if err != nil {
		return nil, err
	}
	return order, nil
}

// GetOrder lets an owner open any order, and a cashier only their own.
func (s *Service) GetOrder(ctx context.Context, id uuid.UUID, onlyCashier *uuid.UUID) (*Order, error) {
	return s.repo.Get(ctx, id, onlyCashier)
}

// mergeLines validates the cart, folds duplicate products together and sorts by product ID.
// A fixed lock order stops two checkouts that share products from deadlocking each other.
func mergeLines(in []LineInput) ([]LineInput, error) {
	if len(in) == 0 {
		return nil, apperr.Invalid("empty_cart", "Add at least one product to the sale.")
	}
	if len(in) > maxLines {
		return nil, apperr.Invalid("too_many_lines", "A sale can have up to 100 different products.")
	}
	qty := make(map[uuid.UUID]int, len(in))
	for _, l := range in {
		if l.Quantity < 1 {
			return nil, apperr.Invalid("bad_quantity", "Quantity must be at least 1.")
		}
		qty[l.ProductID] += l.Quantity
		if qty[l.ProductID] > maxQtyPerProduct {
			return nil, apperr.Invalid("bad_quantity", "Quantity can’t be more than 9,999 per product.")
		}
	}
	out := make([]LineInput, 0, len(qty))
	for id, q := range qty {
		out = append(out, LineInput{ProductID: id, Quantity: q})
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i].ProductID[:], out[j].ProductID[:]) < 0 })
	return out, nil
}
