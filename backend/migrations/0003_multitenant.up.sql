-- Multi-tenant ChloroPOS.
--
--   admin : the company running the service. Manages owners; has no tenant and no way to read a shop's data.
--   owner : one SOHO customer. users.id doubles as the tenant ID on every business table.
--   seat  : a kasir "instance" of an owner (limit users.max_cashiers). Signs in with its own code,
--           never with the owner's password.
--
-- Existing single-shop data moves to the first owner, so upgrading loses nothing.

-- ---------------------------------------------------------------- accounts and kasir seats
ALTER TABLE users
    ADD COLUMN max_cashiers INTEGER NOT NULL DEFAULT 2 CHECK (max_cashiers BETWEEN 0 AND 100);

CREATE TABLE cashier_seats (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID        NOT NULL REFERENCES users (id),
    name                 TEXT        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    -- A kasir code looks like SSSS-XXXX-XXXX. SSSS finds the seat and is not secret; the other
    -- eight characters are the secret and are stored only as an Argon2id hash.
    code_selector        TEXT        NOT NULL CHECK (code_selector ~ '^[0-9A-HJKMNP-TV-Z]{4}$'),
    code_hash            TEXT        NOT NULL,
    is_active            BOOLEAN     NOT NULL DEFAULT TRUE,
    sessions_valid_after TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX cashier_seats_name_key     ON cashier_seats (tenant_id, lower(name));
CREATE UNIQUE INDEX cashier_seats_selector_key ON cashier_seats (tenant_id, code_selector);

-- ---------------------------------------------------------------- tenant column everywhere
ALTER TABLE categories ADD COLUMN tenant_id UUID REFERENCES users (id);
ALTER TABLE products   ADD COLUMN tenant_id UUID REFERENCES users (id);
ALTER TABLE orders
    ADD COLUMN tenant_id    UUID REFERENCES users (id),
    ADD COLUMN seat_id      UUID,
    ADD COLUMN sold_by_name TEXT;
ALTER TABLE order_items ADD COLUMN tenant_id UUID REFERENCES users (id);
ALTER TABLE stock_movements
    ADD COLUMN tenant_id UUID REFERENCES users (id),
    ADD COLUMN seat_id   UUID,
    ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE settings
    ADD COLUMN tenant_id       UUID REFERENCES users (id),
    ADD COLUMN last_receipt_no BIGINT NOT NULL DEFAULT 0;

-- ---------------------------------------------------------------- move existing data to the first owner
DO $$
DECLARE
    legacy UUID;
BEGIN
    SELECT id INTO legacy FROM users WHERE role = 'owner' ORDER BY created_at, id LIMIT 1;

    IF legacy IS NULL THEN
        IF EXISTS (SELECT 1 FROM categories) OR EXISTS (SELECT 1 FROM products) OR EXISTS (SELECT 1 FROM orders) THEN
            RAISE EXCEPTION 'multi-tenant migration: shop data exists but there is no owner account to assign it to';
        END IF;
        DELETE FROM settings;
    ELSE
        -- Old cashier accounts become kasir seats with the same ID, so their orders keep pointing at them.
        -- A seat signs in with a code, not a password, so these seats stay locked ('!' matches nothing)
        -- until the owner generates a code for them.
        INSERT INTO cashier_seats (id, tenant_id, name, code_selector, code_hash, is_active, created_at)
        SELECT id, legacy,
               CASE WHEN dup > 1 THEN left(display_name, 34) || ' #' || n ELSE left(display_name, 40) END,
               upper(lpad(to_hex(n::int), 4, '0')),
               '!', is_active, created_at
          FROM (SELECT u.*,
                       row_number() OVER (ORDER BY u.created_at, u.id) AS n,
                       count(*) OVER (PARTITION BY lower(left(u.display_name, 40))) AS dup
                  FROM users u
                 WHERE u.role = 'cashier') c;

        UPDATE categories SET tenant_id = legacy;
        UPDATE products   SET tenant_id = legacy;
        UPDATE settings   SET tenant_id = legacy;
        UPDATE order_items SET tenant_id = legacy;

        UPDATE orders o
           SET tenant_id    = legacy,
               seat_id      = CASE WHEN u.role = 'cashier' THEN u.id END,
               sold_by_name = u.display_name
          FROM users u
         WHERE u.id = o.cashier_id;

        UPDATE stock_movements m
           SET tenant_id = legacy,
               seat_id   = CASE WHEN u.role = 'cashier' THEN u.id END,
               user_id   = CASE WHEN u.role = 'cashier' THEN NULL ELSE m.user_id END
          FROM users u
         WHERE u.id = m.user_id;
    END IF;
END $$;

ALTER TABLE orders DROP COLUMN cashier_id; -- also drops its foreign key and orders_cashier_created_idx
DELETE FROM users WHERE role = 'cashier';
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'owner'));

-- ---------------------------------------------------------------- constraints, per tenant
ALTER TABLE categories ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX categories_name_key;
CREATE UNIQUE INDEX categories_tenant_name_key ON categories (tenant_id, lower(name));
ALTER TABLE categories ADD CONSTRAINT categories_tenant_id_id_key UNIQUE (tenant_id, id);

ALTER TABLE products ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX products_sku_key;
DROP INDEX products_barcode_key;
CREATE UNIQUE INDEX products_tenant_sku_key ON products (tenant_id, lower(sku));
CREATE UNIQUE INDEX products_tenant_barcode_key ON products (tenant_id, barcode) WHERE barcode IS NOT NULL;
ALTER TABLE products ADD CONSTRAINT products_tenant_id_id_key UNIQUE (tenant_id, id);
-- A product can only use a category of its own shop. The database refuses anything else.
ALTER TABLE products ADD CONSTRAINT products_tenant_category_fkey
    FOREIGN KEY (tenant_id, category_id) REFERENCES categories (tenant_id, id);

-- Receipt numbers count per shop from 1, with no gaps (settings.last_receipt_no is the counter).
ALTER TABLE orders ALTER COLUMN tenant_id SET NOT NULL, ALTER COLUMN sold_by_name SET NOT NULL;
ALTER TABLE orders ALTER COLUMN receipt_no DROP IDENTITY IF EXISTS;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_receipt_no_key;
CREATE UNIQUE INDEX orders_tenant_receipt_key ON orders (tenant_id, receipt_no);
ALTER TABLE orders ADD CONSTRAINT orders_tenant_id_id_key UNIQUE (tenant_id, id);
ALTER TABLE orders ADD CONSTRAINT orders_tenant_seat_fkey
    FOREIGN KEY (tenant_id, seat_id) REFERENCES cashier_seats (tenant_id, id);
DROP INDEX orders_completed_created_idx;
CREATE INDEX orders_tenant_completed_created_idx ON orders (tenant_id, created_at) WHERE status = 'completed';

ALTER TABLE order_items ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE order_items
    ADD CONSTRAINT order_items_tenant_order_fkey
        FOREIGN KEY (tenant_id, order_id) REFERENCES orders (tenant_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT order_items_tenant_product_fkey
        FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id);

ALTER TABLE stock_movements ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE stock_movements
    ADD CONSTRAINT stock_movements_actor_check CHECK (user_id IS NOT NULL OR seat_id IS NOT NULL),
    ADD CONSTRAINT stock_movements_tenant_seat_fkey
        FOREIGN KEY (tenant_id, seat_id) REFERENCES cashier_seats (tenant_id, id),
    ADD CONSTRAINT stock_movements_tenant_product_fkey
        FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id);

-- settings: one row per shop (created on first use), keyed by tenant instead of the old id = 1.
UPDATE settings s
   SET last_receipt_no = COALESCE((SELECT max(o.receipt_no) FROM orders o WHERE o.tenant_id = s.tenant_id), 0);
ALTER TABLE settings DROP COLUMN id; -- also drops its primary key and check
ALTER TABLE settings ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE settings ADD PRIMARY KEY (tenant_id);

-- ---------------------------------------------------------------- row-level security
-- A transaction sees only rows of the tenant named in app.tenant_id (set by the application at the
-- start of each transaction). Without it, no rows are visible and no row can be written.
-- FORCE applies the policy to the table owner as well. Superusers and BYPASSRLS roles still skip it,
-- so the application should connect as an ordinary role (see README).
CREATE FUNCTION app_tenant() RETURNS UUID
    LANGUAGE sql STABLE
    AS $$ SELECT nullif(current_setting('app.tenant_id', true), '')::uuid $$;

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['categories', 'products', 'orders', 'order_items', 'stock_movements', 'settings'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON %I USING (tenant_id = (SELECT app_tenant())) WITH CHECK (tenant_id = (SELECT app_tenant()))',
            t);
    END LOOP;
END $$;
