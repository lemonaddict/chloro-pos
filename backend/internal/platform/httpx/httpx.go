// Package httpx holds small helpers shared by every HTTP handler.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"chloropos/internal/platform/apperr"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func Error(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

var statusByKind = map[apperr.Kind]int{
	apperr.KindInvalid:      http.StatusBadRequest,
	apperr.KindUnauthorized: http.StatusUnauthorized,
	apperr.KindForbidden:    http.StatusForbidden,
	apperr.KindNotFound:     http.StatusNotFound,
	apperr.KindConflict:     http.StatusConflict,
	apperr.KindTooMany:      http.StatusTooManyRequests,
}

// WriteError sends a safe message for known errors. Anything else is logged in full
// and answered with a generic 500, so SQL text and stack details never reach the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if ae, ok := apperr.As(err); ok && ae.Kind != apperr.KindInternal {
		Error(w, statusByKind[ae.Kind], ae.Code, ae.Msg)
		return
	}
	slog.ErrorContext(r.Context(), "request failed", "err", err, "method", r.Method, "path", r.URL.Path)
	Error(w, http.StatusInternalServerError, "internal", "Terjadi gangguan di server. Coba lagi sebentar lagi.")
}

// Decode reads one JSON object: size-capped, JSON content type only, unknown fields rejected.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return apperr.Invalid("bad_content_type", "Kirim permintaan dalam format JSON.")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Invalid("bad_json", "Permintaan tidak bisa dibaca. Periksa isiannya lalu coba lagi.")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return apperr.Invalid("bad_json", "Permintaan harus berisi satu objek JSON.")
	}
	return nil
}

func UUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperr.NotFound("not_found", "Tidak ditemukan.")
	}
	return id, nil
}

// IntQuery parses an integer query value and clamps it to [lo, hi].
func IntQuery(r *http.Request, key string, def, lo, hi int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	return min(max(n, lo), hi)
}

// ClientIP uses the socket address only. X-Forwarded-For is client-controlled unless a
// trusted proxy sets it, so it is deliberately ignored here.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
