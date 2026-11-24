package sales

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"chloropos/internal/platform/apperr"
)

// The use case runs against fakes: no database needed to test pricing, tax and payment rules.
type fakeRepo struct {
	prices   map[uuid.UUID]int64
	inserted *Order
}

func (f *fakeRepo) ReserveStock(_ context.Context, id uuid.UUID, qty int) (*OrderItem, error) {
	price, ok := f.prices[id]
	if !ok {
		return nil, apperr.NotFound("product_not_found", "hilang")
	}
	return &OrderItem{ProductID: id, ProductName: "Barang", UnitPriceCents: price, Quantity: qty}, nil
}

func (f *fakeRepo) Insert(_ context.Context, o *Order) error { f.inserted = o; return nil }

func (f *fakeRepo) Get(context.Context, uuid.UUID, *uuid.UUID) (*Order, error) { return nil, nil }

type passthroughTx struct{}

func (passthroughTx) WithTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type fixedTax struct{ cfg TaxConfig }

func (f fixedTax) Tax(context.Context) (TaxConfig, error) { return f.cfg, nil }

const rp = 100 // one rupiah in minor units

func TestCheckoutPricesFromRepoAndComputesChange(t *testing.T) {
	id := uuid.New()
	svc := NewService(&fakeRepo{prices: map[uuid.UUID]int64{id: 25_000 * rp}}, passthroughTx{}, fixedTax{})

	o, err := svc.Checkout(context.Background(), nil, CheckoutInput{
		Items:               []LineInput{{ProductID: id, Quantity: 2}, {ProductID: id, Quantity: 1}},
		PaymentMethod:       "cash",
		AmountTenderedCents: 100_000 * rp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Items) != 1 || o.Items[0].Quantity != 3 {
		t.Fatalf("duplicate lines should merge, got %+v", o.Items)
	}
	if o.TotalCents != 75_000*rp || o.ChangeCents != 25_000*rp {
		t.Fatalf("want total 75.000 and change 25.000, got %d and %d", o.TotalCents/rp, o.ChangeCents/rp)
	}
}

func TestCheckoutRejectsShortCash(t *testing.T) {
	id := uuid.New()
	svc := NewService(&fakeRepo{prices: map[uuid.UUID]int64{id: 50_000 * rp}}, passthroughTx{}, fixedTax{})

	_, err := svc.Checkout(context.Background(), nil, CheckoutInput{
		Items:               []LineInput{{ProductID: id, Quantity: 1}},
		PaymentMethod:       "cash",
		AmountTenderedCents: 10_000 * rp,
	})
	if ae, ok := apperr.As(err); !ok || ae.Code != "insufficient_payment" {
		t.Fatalf("want insufficient_payment, got %v", err)
	}
}

func TestCheckoutRejectsEmptyCart(t *testing.T) {
	svc := NewService(&fakeRepo{}, passthroughTx{}, fixedTax{})
	if _, err := svc.Checkout(context.Background(), nil, CheckoutInput{PaymentMethod: "cash"}); err == nil {
		t.Fatal("expected an error for an empty cart")
	}
}

func TestCheckoutRecordsTheKasirSeat(t *testing.T) {
	id, seat := uuid.New(), uuid.New()
	repo := &fakeRepo{prices: map[uuid.UUID]int64{id: 10_000 * rp}}
	svc := NewService(repo, passthroughTx{}, fixedTax{})

	if _, err := svc.Checkout(context.Background(), &seat, CheckoutInput{
		Items: []LineInput{{ProductID: id, Quantity: 1}}, PaymentMethod: "qris",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.inserted.SeatID == nil || *repo.inserted.SeatID != seat {
		t.Fatalf("order should carry the kasir seat, got %v", repo.inserted.SeatID)
	}

	if _, err := svc.Checkout(context.Background(), nil, CheckoutInput{
		Items: []LineInput{{ProductID: id, Quantity: 1}}, PaymentMethod: "qris",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.inserted.SeatID != nil {
		t.Fatalf("an owner's sale has no seat, got %v", repo.inserted.SeatID)
	}
}

func TestExclusiveTaxIsAddedOnTop(t *testing.T) {
	id := uuid.New()
	cfg := TaxConfig{Enabled: true, Label: "PPN", RateBPS: 1100, Inclusive: false}
	svc := NewService(&fakeRepo{prices: map[uuid.UUID]int64{id: 100_000 * rp}}, passthroughTx{}, fixedTax{cfg})

	o, err := svc.Checkout(context.Background(), nil, CheckoutInput{
		Items: []LineInput{{ProductID: id, Quantity: 1}}, PaymentMethod: "qris",
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.SubtotalCents != 100_000*rp || o.TaxCents != 11_000*rp || o.TotalCents != 111_000*rp {
		t.Fatalf("got subtotal %d, tax %d, total %d", o.SubtotalCents/rp, o.TaxCents/rp, o.TotalCents/rp)
	}
	if o.TaxLabel != "PPN" || o.TaxRateBPS != 1100 || o.TaxInclusive {
		t.Fatalf("tax snapshot not stored on the order: %+v", o)
	}
}

func TestInclusiveTaxIsPartOfThePrice(t *testing.T) {
	tax, total := ComputeTax(111_000*rp, TaxConfig{Enabled: true, RateBPS: 1100, Inclusive: true})
	if tax != 11_000*rp || total != 111_000*rp {
		t.Fatalf("got tax %d, total %d", tax/rp, total/rp)
	}
}

func TestTaxRoundsToWholeRupiah(t *testing.T) {
	// 11% of 12.345 = 1.357,95 -> 1.358
	tax, _ := ComputeTax(12_345*rp, TaxConfig{Enabled: true, RateBPS: 1100, Inclusive: false})
	if tax != 1_358*rp {
		t.Fatalf("got %d", tax/rp)
	}
}

func TestNoTaxWhenDisabled(t *testing.T) {
	tax, total := ComputeTax(50_000*rp, TaxConfig{Enabled: false, RateBPS: 1100})
	if tax != 0 || total != 50_000*rp {
		t.Fatalf("got tax %d, total %d", tax, total)
	}
}
