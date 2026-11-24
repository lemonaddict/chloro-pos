// Package tenancy carries the tenant a request acts for: the SOHO customer whose shop it is.
//
// The HTTP layer sets it once, right after authentication, from the signed session. The database
// layer reads it to scope every transaction (row-level security), and repositories read it to scope
// every query. Nothing in a request body or URL can change it.
package tenancy

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrMissing means code that needs a tenant ran without one. That is a bug, not a user error:
// the query must not run, so repositories return this instead of falling back to "all tenants".
var ErrMissing = errors.New("tenancy: no tenant in context")

type ctxKey struct{}

func With(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func From(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// Require is what repositories call before touching a tenant's data.
func Require(ctx context.Context) (uuid.UUID, error) {
	if id, ok := From(ctx); ok {
		return id, nil
	}
	return uuid.Nil, ErrMissing
}
