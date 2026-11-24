package tenancy

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestRequireFailsClosed(t *testing.T) {
	if _, err := Require(context.Background()); !errors.Is(err, ErrMissing) {
		t.Fatalf("no tenant must be an error, got %v", err)
	}
	if _, err := Require(With(context.Background(), uuid.Nil)); !errors.Is(err, ErrMissing) {
		t.Fatalf("the zero UUID is not a tenant, got %v", err)
	}
	id := uuid.New()
	if got, err := Require(With(context.Background(), id)); err != nil || got != id {
		t.Fatalf("want %v, got %v (%v)", id, got, err)
	}
}
