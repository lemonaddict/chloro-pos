package sales

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"sohopos/internal/platform/apperr"
)

// The use case runs against fakes: no database needed to test pricing and payment rules.
type fakeRepo struct {
	prices   map[uuid.UUID]int64
	inserted *Order
}

func (f *fakeRepo) ReserveStock(_ context.Context, id uuid.UUID, qty int) (*OrderItem, error) {
	price, ok := f.prices[id]
	if !ok {
		return nil, apperr.NotFound("product_not_found", "gone")
	}
	return &OrderItem{ProductID: id, ProductName: "Item", UnitPriceCents: price, Quantity: qty}, nil
}

func (f *fakeRepo) Insert(_ context.Context, o *Order) error { f.inserted = o; return nil }

func (f *fakeRepo) Get(context.Context, uuid.UUID, *uuid.UUID) (*Order, error) { return nil, nil }

type passthroughTx struct{}

func (passthroughTx) WithTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func TestCheckoutPricesFromRepoAndComputesChange(t *testing.T) {
	id := uuid.New()
	svc := NewService(&fakeRepo{prices: map[uuid.UUID]int64{id: 250}}, passthroughTx{})

	o, err := svc.Checkout(context.Background(), uuid.New(), CheckoutInput{
		Items:               []LineInput{{ProductID: id, Quantity: 2}, {ProductID: id, Quantity: 1}},
		PaymentMethod:       "cash",
		AmountTenderedCents: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Items) != 1 || o.Items[0].Quantity != 3 {
		t.Fatalf("duplicate lines should merge, got %+v", o.Items)
	}
	if o.TotalCents != 750 || o.ChangeCents != 250 {
		t.Fatalf("want total 750 and change 250, got %d and %d", o.TotalCents, o.ChangeCents)
	}
}

func TestCheckoutRejectsShortCash(t *testing.T) {
	id := uuid.New()
	svc := NewService(&fakeRepo{prices: map[uuid.UUID]int64{id: 500}}, passthroughTx{})

	_, err := svc.Checkout(context.Background(), uuid.New(), CheckoutInput{
		Items:               []LineInput{{ProductID: id, Quantity: 1}},
		PaymentMethod:       "cash",
		AmountTenderedCents: 100,
	})
	if ae, ok := apperr.As(err); !ok || ae.Code != "insufficient_payment" {
		t.Fatalf("want insufficient_payment, got %v", err)
	}
}

func TestCheckoutRejectsEmptyCart(t *testing.T) {
	svc := NewService(&fakeRepo{}, passthroughTx{})
	if _, err := svc.Checkout(context.Background(), uuid.New(), CheckoutInput{PaymentMethod: "cash"}); err == nil {
		t.Fatal("expected an error for an empty cart")
	}
}
