package identity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/auth"
)

// ---- fakes: the use cases run without a database ----

type passthroughTx struct{}

func (passthroughTx) WithTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type fakeRepo struct {
	users map[uuid.UUID]*User
	seats map[uuid.UUID]*Seat
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: map[uuid.UUID]*User{}, seats: map[uuid.UUID]*Seat{}}
}

func (f *fakeRepo) FindByUsername(_ context.Context, name string) (*User, error) {
	for _, u := range f.users {
		if strings.EqualFold(u.Username, name) {
			return u, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) CountAdmins(context.Context) (int64, error)             { return 0, nil }
func (f *fakeRepo) Create(context.Context, *User) error                     { return nil }
func (f *fakeRepo) ListTenants(context.Context, int) ([]Tenant, error)      { return nil, nil }
func (f *fakeRepo) SetTenantPassword(context.Context, uuid.UUID, string) error { return nil }
func (f *fakeRepo) UpdateTenant(context.Context, uuid.UUID, string, bool, *int) error {
	return nil
}

func (f *fakeRepo) activeSeats(tenantID uuid.UUID) int {
	n := 0
	for _, s := range f.seats {
		if s.TenantID == tenantID && s.IsActive {
			n++
		}
	}
	return n
}

func (f *fakeRepo) FindSeatBySelector(_ context.Context, tenantID uuid.UUID, selector string) (*Seat, error) {
	for _, s := range f.seats {
		if s.TenantID == tenantID && s.Selector == selector {
			return s, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) FindSeatForSession(_ context.Context, id uuid.UUID) (*Seat, error) {
	s, ok := f.seats[id]
	if !ok {
		return nil, ErrNotFound
	}
	owner := f.users[s.TenantID]
	out := *s
	out.IsActive = s.IsActive && owner.IsActive
	out.TenantUsername = owner.Username
	return &out, nil
}

func (f *fakeRepo) FindSeat(_ context.Context, tenantID, id uuid.UUID) (*Seat, error) {
	if s, ok := f.seats[id]; ok && s.TenantID == tenantID {
		return s, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) ListSeats(_ context.Context, tenantID uuid.UUID) ([]Seat, error) {
	var out []Seat
	for _, s := range f.seats {
		if s.TenantID == tenantID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (f *fakeRepo) CreateSeat(_ context.Context, s *Seat) error {
	owner, ok := f.users[s.TenantID]
	if !ok {
		return ErrNotFound
	}
	if f.activeSeats(s.TenantID) >= owner.MaxCashiers {
		return &SeatLimitError{Limit: owner.MaxCashiers}
	}
	s.ID, s.IsActive, s.SessionsValidAfter, s.CreatedAt = uuid.New(), true, time.Now().Add(-time.Minute), time.Now()
	cp := *s
	f.seats[s.ID] = &cp
	return nil
}

func (f *fakeRepo) UpdateSeat(_ context.Context, tenantID, id uuid.UUID, name string, active bool) error {
	s, ok := f.seats[id]
	if !ok || s.TenantID != tenantID {
		return ErrNotFound
	}
	if active && !s.IsActive && f.activeSeats(tenantID) >= f.users[tenantID].MaxCashiers {
		return &SeatLimitError{Limit: f.users[tenantID].MaxCashiers}
	}
	s.Name, s.IsActive = name, active
	return nil
}

func (f *fakeRepo) SetSeatCode(_ context.Context, tenantID, id uuid.UUID, selector, hash string) error {
	s, ok := f.seats[id]
	if !ok || s.TenantID != tenantID {
		return ErrNotFound
	}
	s.Selector, s.CodeHash = selector, hash
	return nil
}

// ---- helpers ----

const ownerPassword = "owner-password-123"

type fixture struct {
	svc    *Service
	repo   *fakeRepo
	tokens *auth.TokenManager
	owner  *User
	admin  *User
}

func newFixture(t *testing.T, maxCashiers int) *fixture {
	t.Helper()
	hash, err := auth.HashPassword(ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	owner := &User{ID: uuid.New(), Username: "tmb", DisplayName: "Toko Maju Bersama", Role: RoleOwner,
		PasswordHash: hash, IsActive: true, SessionsValidAfter: time.Now().Add(-time.Hour), MaxCashiers: maxCashiers}
	admin := &User{ID: uuid.New(), Username: "isp", DisplayName: "Admin", Role: RoleAdmin,
		PasswordHash: hash, IsActive: true, SessionsValidAfter: time.Now().Add(-time.Hour)}
	repo.users[owner.ID], repo.users[admin.ID] = owner, admin

	tokens := auth.NewTokenManager([]byte(strings.Repeat("k", 32)), time.Hour)
	return &fixture{svc: NewService(repo, passthroughTx{}, tokens), repo: repo, tokens: tokens, owner: owner, admin: admin}
}

func errCode(err error) string {
	if ae, ok := apperr.As(err); ok {
		return ae.Code
	}
	return ""
}

// ---- kasir codes ----

func TestCashierCodeRoundTrip(t *testing.T) {
	code, err := newCashierCode()
	if err != nil {
		t.Fatal(err)
	}
	shown := code.String()
	if len(shown) != 14 || shown[4] != '-' || shown[9] != '-' {
		t.Fatalf("want SSSS-XXXX-XXXX, got %q", shown)
	}
	for _, typed := range []string{shown, strings.ToLower(shown), strings.ReplaceAll(shown, "-", ""), " " + shown + " "} {
		got, ok := parseCashierCode(typed)
		if !ok || got != code {
			t.Fatalf("typing %q should give back the code, got %+v ok=%v", typed, got, ok)
		}
	}
}

func TestParseCashierCodeFixesCommonSlipsAndRejectsJunk(t *testing.T) {
	got, ok := parseCashierCode("oi1l-abcd-efgh")
	if !ok || got.Selector != "0111" || got.Secret != "ABCDEFGH" {
		t.Fatalf("O, I and L should read as 0, 1 and 1: got %+v ok=%v", got, ok)
	}
	for _, bad := range []string{"", "ABCD-EFGH", "ABCD-EFGH-IJKM-NPQR", "ABCD-EFGH-UUUU", "ÄBCD-EFGH-JKMN", strings.Repeat("A", 100)} {
		if _, ok := parseCashierCode(bad); ok {
			t.Fatalf("%q should be rejected", bad)
		}
	}
}

// ---- kasir sign-in ----

func TestCashierSignsInWithItsOwnCodeNeverTheOwnersPassword(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	_, code, err := f.svc.CreateSeat(ctx, f.owner.ID, "Kasir 1")
	if err != nil {
		t.Fatal(err)
	}

	sess, err := f.svc.LoginCashier(ctx, "TMB", code)
	if err != nil {
		t.Fatalf("the seat's code should sign in: %v", err)
	}
	if sess.Identity.Role != RoleCashier || sess.Identity.DisplayName != "Kasir 1" || sess.Identity.Username != "tmb" {
		t.Fatalf("unexpected identity %+v", sess.Identity)
	}

	if _, err := f.svc.LoginCashier(ctx, "tmb", ownerPassword); errCode(err) != "invalid_credentials" {
		t.Fatalf("the owner's password must not open the kasir door, got %v", err)
	}
	if _, err := f.svc.Login(ctx, "tmb", code); errCode(err) != "invalid_credentials" {
		t.Fatalf("a kasir code must not open the owner's account, got %v", err)
	}
	last := "A"
	if strings.HasSuffix(code, "A") {
		last = "B"
	}
	wrong := code[:len(code)-1] + last
	if _, err := f.svc.LoginCashier(ctx, "tmb", wrong); errCode(err) != "invalid_credentials" {
		t.Fatalf("a wrong code must fail, got %v", err)
	}
}

func TestResetSeatCodeLocksOutTheOldCode(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	seat, oldCode, err := f.svc.CreateSeat(ctx, f.owner.ID, "Kasir 1")
	if err != nil {
		t.Fatal(err)
	}
	_, newCode, err := f.svc.ResetSeatCode(ctx, f.owner.ID, seat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.LoginCashier(ctx, "tmb", oldCode); err == nil {
		t.Fatal("the old code should stop working")
	}
	if _, err := f.svc.LoginCashier(ctx, "tmb", newCode); err != nil {
		t.Fatalf("the new code should work: %v", err)
	}
}

func TestAnotherShopCannotTouchMySeats(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	seat, _, err := f.svc.CreateSeat(ctx, f.owner.ID, "Kasir 1")
	if err != nil {
		t.Fatal(err)
	}
	other := uuid.New()
	f.repo.users[other] = &User{ID: other, Username: "other", Role: RoleOwner, IsActive: true, MaxCashiers: 2}

	if _, err := f.svc.UpdateSeat(ctx, other, seat.ID, "Dibajak", false); errCode(err) != "seat_not_found" {
		t.Fatalf("a seat of another shop must read as not found, got %v", err)
	}
	if _, _, err := f.svc.ResetSeatCode(ctx, other, seat.ID); errCode(err) != "seat_not_found" {
		t.Fatalf("resetting another shop's code must read as not found, got %v", err)
	}
}

// ---- seat limit ----

func TestSeatLimit(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	first, _, err := f.svc.CreateSeat(ctx, f.owner.ID, "Kasir 1")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.CreateSeat(ctx, f.owner.ID, "Kasir 2"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.CreateSeat(ctx, f.owner.ID, "Kasir 3"); errCode(err) != "seat_limit" {
		t.Fatalf("a third seat should hit the limit of 2, got %v", err)
	}

	// Switching one off frees a seat; switching it back on takes it again.
	if _, err := f.svc.UpdateSeat(ctx, f.owner.ID, first.ID, "Kasir 1", false); err != nil {
		t.Fatal(err)
	}
	third, _, err := f.svc.CreateSeat(ctx, f.owner.ID, "Kasir 3")
	if err != nil {
		t.Fatalf("a freed seat should be usable: %v", err)
	}
	if _, err := f.svc.UpdateSeat(ctx, f.owner.ID, first.ID, "Kasir 1", true); errCode(err) != "seat_limit" {
		t.Fatalf("reactivating over the limit must be refused, got %v", err)
	}
	list, err := f.svc.ListSeats(ctx, f.owner.ID)
	if err != nil || list.Limit != 2 || list.Active != 2 || len(list.Seats) != 3 {
		t.Fatalf("want limit 2, 2 active, 3 seats, got %+v err=%v (third=%v)", list, err, third.ID)
	}
}

// ---- sessions ----

func claimsOf(t *testing.T, f *fixture, token string) *auth.Claims {
	t.Helper()
	c, err := f.tokens.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSessionsFollowTheAccountsState(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	seat, code, err := f.svc.CreateSeat(ctx, f.owner.ID, "Kasir 1")
	if err != nil {
		t.Fatal(err)
	}
	cashier, err := f.svc.LoginCashier(ctx, "tmb", code)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := f.svc.Login(ctx, "tmb", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := f.svc.Login(ctx, "isp", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}

	cc, oc, ac := claimsOf(t, f, cashier.Token), claimsOf(t, f, owner.Token), claimsOf(t, f, admin.Token)
	if cc.TID != f.owner.ID.String() || oc.TID != f.owner.ID.String() || ac.TID != "" {
		t.Fatalf("tenant claims wrong: cashier %q owner %q admin %q", cc.TID, oc.TID, ac.TID)
	}
	for name, c := range map[string]*auth.Claims{"cashier": cc, "owner": oc, "admin": ac} {
		if !f.svc.SessionValid(ctx, c) {
			t.Fatalf("a fresh %s session should be valid", name)
		}
	}

	// A token that names another shop is refused, even though its signature is genuine.
	forged := *cc
	forged.TID = uuid.New().String()
	if f.svc.SessionValid(ctx, &forged) {
		t.Fatal("a kasir token for the wrong shop must not pass")
	}
	// The admin has no shop.
	adminWithShop := *ac
	adminWithShop.TID = f.owner.ID.String()
	if f.svc.SessionValid(ctx, &adminWithShop) {
		t.Fatal("an admin token must not carry a shop")
	}
	// An owner token cannot borrow a seat's ID, nor the other way round.
	swapped := *cc
	swapped.Role = RoleOwner
	if f.svc.SessionValid(ctx, &swapped) {
		t.Fatal("a seat ID presented as an owner must not pass")
	}

	// Switching the seat off ends its session.
	if _, err := f.svc.UpdateSeat(ctx, f.owner.ID, seat.ID, "Kasir 1", false); err != nil {
		t.Fatal(err)
	}
	if f.svc.SessionValid(ctx, cc) {
		t.Fatal("a deactivated seat's session must end")
	}

	// Switching the shop off ends the kasir session too, even for an active seat.
	if _, err := f.svc.UpdateSeat(ctx, f.owner.ID, seat.ID, "Kasir 1", true); err != nil {
		t.Fatal(err)
	}
	f.owner.IsActive = false
	f.svc.forget(f.owner.ID)
	f.svc.forget(seat.ID)
	if f.svc.SessionValid(ctx, cc) || f.svc.SessionValid(ctx, oc) {
		t.Fatal("a deactivated shop's sessions must end, kasir included")
	}
}
