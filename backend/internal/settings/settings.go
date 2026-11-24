// Package settings holds the business profile printed on receipts, and the tax rules.
// Each shop (tenant) has one row, created the first time the shop needs it.
package settings

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/database"
	"chloropos/internal/platform/httpx"
	"chloropos/internal/platform/middleware"
	"chloropos/internal/platform/tenancy"
)

type Settings struct {
	BusinessName  string `json:"business_name"`
	LegalName     string `json:"legal_name"`
	Address       string `json:"address"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	NPWP          string `json:"npwp"` // digits only: 15 (old format) or 16 (NIK-based)
	ReceiptFooter string `json:"receipt_footer"`
	TaxEnabled    bool   `json:"tax_enabled"`
	TaxLabel      string `json:"tax_label"`
	TaxRateBPS    int    `json:"tax_rate_bps"` // 1100 = 11%
	TaxInclusive  bool   `json:"tax_inclusive"`
}

type Repository interface {
	Get(ctx context.Context) (*Settings, error)
	Update(ctx context.Context, s Settings) error
}

var (
	phoneRe = regexp.MustCompile(`^[0-9+()\-\s]*$`)
	emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	npwpRe  = regexp.MustCompile(`^([0-9]{15}|[0-9]{16})?$`)
)

type Service struct {
	repo Repository
	tx   database.Transactor
}

func NewService(repo Repository, tx database.Transactor) *Service { return &Service{repo: repo, tx: tx} }

// Get and Update run in a transaction, which is what pins them to the shop's own row.
func (s *Service) Get(ctx context.Context) (*Settings, error) {
	return database.InTx(ctx, s.tx, s.repo.Get)
}

func (s *Service) Update(ctx context.Context, in Settings) (*Settings, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err := s.tx.WithTx(ctx, func(ctx context.Context) error { return s.repo.Update(ctx, in) })
	if err != nil {
		return nil, err
	}
	return &in, nil
}

// text trims, unifies line breaks, caps the length and rejects control characters.
func text(field, value string, limit int, multiline bool) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
	if utf8.RuneCountInString(value) > limit {
		return "", apperr.Invalid("too_long", field+" maksimal "+strconv.Itoa(limit)+" karakter.")
	}
	for _, r := range value {
		if unicode.IsControl(r) && !(multiline && r == '\n') {
			return "", apperr.Invalid("bad_characters", field+" memuat karakter yang tidak diperbolehkan.")
		}
	}
	return value, nil
}

func (in *Settings) normalize() error {
	var err error
	if in.BusinessName, err = text("Nama usaha", in.BusinessName, 80, false); err != nil {
		return err
	}
	if in.BusinessName == "" {
		return apperr.Invalid("name_required", "Nama usaha wajib diisi.")
	}
	if in.LegalName, err = text("Nama badan usaha", in.LegalName, 120, false); err != nil {
		return err
	}
	if in.Address, err = text("Alamat", in.Address, 300, true); err != nil {
		return err
	}
	if in.Phone, err = text("Nomor telepon", in.Phone, 30, false); err != nil {
		return err
	}
	if !phoneRe.MatchString(in.Phone) {
		return apperr.Invalid("bad_phone", "Nomor telepon hanya boleh berisi angka, spasi, +, - dan tanda kurung.")
	}
	if in.Email, err = text("Email", in.Email, 120, false); err != nil {
		return err
	}
	if in.Email != "" && !emailRe.MatchString(in.Email) {
		return apperr.Invalid("bad_email", "Format email belum benar.")
	}
	npwp := strings.NewReplacer(".", "", "-", "", " ", "").Replace(in.NPWP)
	if !npwpRe.MatchString(npwp) {
		return apperr.Invalid("bad_npwp", "NPWP harus 15 atau 16 angka.")
	}
	in.NPWP = npwp
	if in.ReceiptFooter, err = text("Pesan di struk", in.ReceiptFooter, 200, true); err != nil {
		return err
	}
	if in.TaxLabel, err = text("Nama pajak", in.TaxLabel, 20, false); err != nil {
		return err
	}
	if in.TaxLabel == "" {
		return apperr.Invalid("tax_label_required", "Nama pajak wajib diisi, misalnya PPN.")
	}
	if in.TaxRateBPS < 0 || in.TaxRateBPS > 10000 {
		return apperr.Invalid("bad_tax_rate", "Tarif pajak harus antara 0% dan 100%.")
	}
	return nil
}

// ---- Postgres adapter ----

type PGRepository struct{ db *database.Store }

func NewPGRepository(db *database.Store) *PGRepository { return &PGRepository{db: db} }

const settingsSelect = `SELECT business_name, legal_name, address, phone, email, npwp, receipt_footer,
	tax_enabled, tax_label, tax_rate_bps, tax_inclusive
	FROM settings WHERE tenant_id = $1`

func (r *PGRepository) load(ctx context.Context, tenantID uuid.UUID) (*Settings, error) {
	var s Settings
	err := r.db.DB(ctx).QueryRow(ctx, settingsSelect, tenantID).
		Scan(&s.BusinessName, &s.LegalName, &s.Address, &s.Phone, &s.Email, &s.NPWP, &s.ReceiptFooter,
			&s.TaxEnabled, &s.TaxLabel, &s.TaxRateBPS, &s.TaxInclusive)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PGRepository) Get(ctx context.Context) (*Settings, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	s, err := r.load(ctx, tid)
	if errors.Is(err, pgx.ErrNoRows) {
		// A new shop's first visit: start it with defaults, named after the account.
		if _, err := r.db.DB(ctx).Exec(ctx,
			`INSERT INTO settings (tenant_id, business_name)
			 SELECT id, display_name FROM users WHERE id = $1
			 ON CONFLICT (tenant_id) DO NOTHING`, tid); err != nil {
			return nil, err
		}
		return r.load(ctx, tid)
	}
	return s, err
}

func (r *PGRepository) Update(ctx context.Context, s Settings) error {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return err
	}
	_, err = r.db.DB(ctx).Exec(ctx,
		`INSERT INTO settings (tenant_id, business_name, legal_name, address, phone, email, npwp, receipt_footer,
		                       tax_enabled, tax_label, tax_rate_bps, tax_inclusive)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (tenant_id) DO UPDATE
		    SET business_name = EXCLUDED.business_name, legal_name = EXCLUDED.legal_name,
		        address = EXCLUDED.address, phone = EXCLUDED.phone, email = EXCLUDED.email,
		        npwp = EXCLUDED.npwp, receipt_footer = EXCLUDED.receipt_footer,
		        tax_enabled = EXCLUDED.tax_enabled, tax_label = EXCLUDED.tax_label,
		        tax_rate_bps = EXCLUDED.tax_rate_bps, tax_inclusive = EXCLUDED.tax_inclusive,
		        updated_at = now()`,
		tid, s.BusinessName, s.LegalName, s.Address, s.Phone, s.Email, s.NPWP, s.ReceiptFooter,
		s.TaxEnabled, s.TaxLabel, s.TaxRateBPS, s.TaxInclusive)
	return err
}

// ---- HTTP ----

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes run behind Authenticate, CSRF and TenantScope. The shop's owner and its kasir may read
// (receipts need it); only the owner writes. There is no public endpoint: with many shops on one
// server, the sign-in page has no single business name to show.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/settings", h.get)
	r.With(middleware.RequireRole(auth.RoleOwner)).Put("/settings", h.update)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Get(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var in Settings
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s, err := h.svc.Update(r.Context(), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
