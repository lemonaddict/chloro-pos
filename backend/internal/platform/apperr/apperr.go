// Package apperr carries a safe, typed error from use cases up to the HTTP layer.
package apperr

import "errors"

type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindNotFound
	KindConflict
	KindUnauthorized
	KindForbidden
	KindTooMany
)

// Error is safe to show to a user: Code is stable and machine-readable, Msg is plain language.
type Error struct {
	Kind Kind
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Code + ": " + e.Msg }

func New(kind Kind, code, msg string) *Error { return &Error{Kind: kind, Code: code, Msg: msg} }
func Invalid(code, msg string) *Error        { return New(KindInvalid, code, msg) }
func NotFound(code, msg string) *Error       { return New(KindNotFound, code, msg) }
func Conflict(code, msg string) *Error       { return New(KindConflict, code, msg) }
func Unauthorized(code, msg string) *Error   { return New(KindUnauthorized, code, msg) }
func Forbidden(code, msg string) *Error      { return New(KindForbidden, code, msg) }

// As reports whether err is (or wraps) an *Error.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
