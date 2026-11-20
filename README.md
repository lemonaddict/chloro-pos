# SOHO POS

A small point-of-sale app for home and small-office shops. Go + PostgreSQL on the back, Svelte + Tailwind on the front, one binary in production.

> **Status:** written without a compiler in reach (the build sandbox had no Go toolchain or network). Expect to fix a few small things on the first `go build` / `npm run build`. The sales use-case tests need only `go test ./internal/sales/...`.

## Layout

```
sohopos/
├── backend/
│   ├── cmd/api/main.go            entry point: config, pool, migrate, serve, graceful shutdown
│   ├── migrations/                0001_schema.sql (the schema) + embed.go (ships inside the binary)
│   └── internal/
│       ├── app/                   composition root: builds modules, mounts routes, serves the SPA
│       ├── platform/              shared kernel: config, database, auth, middleware, httpx, apperr
│       ├── identity/              users, sign-in, sessions
│       ├── catalog/               products, categories, stock adjustments   (full sample module)
│       ├── sales/                 checkout transaction, receipts
│       └── reporting/             daily revenue, fast-moving items
├── frontend/                      Vite + Svelte 5 + Tailwind v4
│   └── src/{lib,components,routes}
├── Dockerfile                     node build -> go build -> alpine runtime
├── docker-compose.yml             db + app (optional)
└── .env.example
```

Each module has the same four files: `domain.go` (entities and the repository interface), `service.go` (use cases), `repository_pg.go` (SQL), `handler.go` (HTTP). Modules never import each other; only `internal/app` wires them together.

## Run on Windows (no Docker)

You need Go 1.23+, Node 20+, and PostgreSQL 16+ (the EDB installer is fine).

1. Create the database (PowerShell, in the PostgreSQL `bin` folder or with it on PATH):
   ```powershell
   psql -U postgres -c "CREATE USER sohopos WITH PASSWORD 'choose-letters-and-digits'"
   psql -U postgres -c "CREATE DATABASE sohopos OWNER sohopos"
   ```
2. Copy `.env.example` to `.env` in the project root and fill in `DATABASE_URL`, `JWT_SECRET` (the file shows how to generate one) and `BOOTSTRAP_OWNER_PASSWORD`.
3. Backend (migrations run automatically on start):
   ```powershell
   cd backend
   go mod tidy
   go test ./internal/sales/...
   go run ./cmd/api
   ```
4. Frontend, in a second terminal:
   ```powershell
   cd frontend
   npm install
   npm run dev
   ```
5. Open http://localhost:5173 and sign in as the bootstrap owner. Then remove the two `BOOTSTRAP_OWNER_*` lines from `.env`.

Optional `frontend/.env.local`: copy `frontend/.env.example` to set the shop name and currency shown on screen and receipts.

## Run with Docker (optional)

```powershell
copy .env.example .env      # fill in POSTGRES_PASSWORD, JWT_SECRET, BOOTSTRAP_OWNER_PASSWORD
docker compose up --build
```
Open http://localhost:8080. The Go server serves the built Svelte app itself, so no Nginx is needed. PostgreSQL is not exposed outside the compose network.

## Keyboard shortcuts (Sell screen)

F1 charge, F2 search. A barcode scanner works as-is: it types into the search box and presses Enter.

## Security notes

| Concern | What is done |
|---|---|
| Passwords | Argon2id (OWASP minimum profile), PHC format, constant-time compare, dummy hash for unknown users |
| Sessions | HS256 JWT with pinned algorithm, issuer, audience and expiry, in an httpOnly, Secure, SameSite=Strict cookie |
| CSRF | A random token lives inside the signed session and must come back in `X-CSRF-Token` on every change; Origin is checked too |
| SQL injection | Parameterized pgx queries only; LIKE wildcards in search input are escaped |
| Authorization | Roles enforced in the API (`owner` / `cashier`); the seller comes from the session; order lookups are scoped to the owner of the row |
| Pricing | Clients send product IDs and quantities, never prices; the server prices every line in the checkout transaction |
| Brute force | Per-IP and per-username limits on sign-in, plus a general per-IP API limit |
| Headers | CSP (no inline script/style), HSTS when cookies are Secure, nosniff, frame deny, no-referrer, COOP, no-store on API responses |
| Input | 1 MB body cap, JSON-only, unknown fields rejected, list pages capped at 100 |
| Errors | Known errors return plain-language messages; everything else is logged in full and returns a generic 500 |

Known trade-offs: sessions are stateless, so signing out clears the cookie but a copied token stays valid until it expires (10 hours by default; set `SESSION_HOURS`). A role change also applies from the next sign-in. Rate limits are in memory, which is right for one server.

Using the app over plain http on a shop LAN (for example `http://192.168.1.20:8080`) means Secure cookies won't be stored. Either put HTTPS in front (Caddy is the easy option) or set `COOKIE_SECURE=false` knowing the session then travels unencrypted.

## Scaling and extraction

- The pool is bounded (`DB_MAX_CONNS`), with lifetimes, health checks and server-side statement and idle-in-transaction timeouts. Every query takes the request context, and each request has a 30 s deadline.
- Checkout reserves stock with one atomic `UPDATE ... WHERE stock_qty >= n`, in a fixed product order, so concurrent sales cannot oversell or deadlock.
- To extract a module later: its `domain.go` is the contract, its `repository_pg.go` is the only code that touches its tables, and `internal/app` is the only place that connects modules. `reporting` is the one that reads other modules' tables; feed it from order events when it moves out.
- `sqlc` can replace the hand-written queries one repository at a time without touching services or handlers.

## Not built yet

User management screens (the API exists: `POST /users`, owner only), voids and refunds, tax and discounts, a thermal-printer layout, and offline queuing. Money is stored as integer minor units, so adding them is straightforward.
