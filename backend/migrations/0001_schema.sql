-- SOHO POS schema (PostgreSQL 13+).
-- Money is stored as integer minor units (cents). Never floats.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT        NOT NULL CHECK (char_length(username) BETWEEN 3 AND 40),
    display_name  TEXT        NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 80),
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL CHECK (role IN ('owner', 'cashier')),
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_username_key ON users (lower(username));

CREATE TABLE categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX categories_name_key ON categories (lower(name));

CREATE TABLE products (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id         UUID REFERENCES categories (id) ON DELETE SET NULL,
    sku                 TEXT        NOT NULL CHECK (char_length(sku) BETWEEN 1 AND 40),
    barcode             TEXT        CHECK (char_length(barcode) BETWEEN 1 AND 64),
    name                TEXT        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    price_cents         BIGINT      NOT NULL CHECK (price_cents >= 0),
    stock_qty           INTEGER     NOT NULL DEFAULT 0 CHECK (stock_qty >= 0),
    low_stock_threshold INTEGER     NOT NULL DEFAULT 5 CHECK (low_stock_threshold >= 0),
    is_active           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX products_sku_key     ON products (lower(sku));
CREATE UNIQUE INDEX products_barcode_key ON products (barcode) WHERE barcode IS NOT NULL;
CREATE INDEX products_category_idx       ON products (category_id);
CREATE INDEX products_name_trgm_idx      ON products USING gin (name gin_trgm_ops);

CREATE TABLE orders (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    receipt_no            BIGINT      GENERATED ALWAYS AS IDENTITY UNIQUE,
    cashier_id            UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status                TEXT        NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'void')),
    total_cents           BIGINT      NOT NULL CHECK (total_cents >= 0),
    payment_method        TEXT        NOT NULL CHECK (payment_method IN ('cash', 'card', 'transfer')),
    amount_tendered_cents BIGINT      NOT NULL CHECK (amount_tendered_cents >= total_cents),
    change_cents          BIGINT      NOT NULL CHECK (change_cents = amount_tendered_cents - total_cents),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Reports only read completed sales, so keep that index small.
CREATE INDEX orders_completed_created_idx ON orders (created_at) WHERE status = 'completed';
CREATE INDEX orders_cashier_created_idx   ON orders (cashier_id, created_at DESC);

-- Product name and price are copied onto the line so old receipts never change.
CREATE TABLE order_items (
    id               BIGINT  GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id         UUID    NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id       UUID    NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    product_name     TEXT    NOT NULL,
    unit_price_cents BIGINT  NOT NULL CHECK (unit_price_cents >= 0),
    quantity         INTEGER NOT NULL CHECK (quantity > 0),
    line_total_cents BIGINT  NOT NULL CHECK (line_total_cents = unit_price_cents * quantity)
);
CREATE INDEX order_items_order_idx   ON order_items (order_id);
CREATE INDEX order_items_product_idx ON order_items (product_id);

-- Audit trail for every stock change: sales, restocks, corrections, damage.
CREATE TABLE stock_movements (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    delta      INTEGER     NOT NULL CHECK (delta <> 0),
    reason     TEXT        NOT NULL CHECK (reason IN ('sale', 'restock', 'correction', 'damage')),
    order_id   UUID        REFERENCES orders (id) ON DELETE SET NULL,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX stock_movements_product_idx ON stock_movements (product_id, created_at DESC);
