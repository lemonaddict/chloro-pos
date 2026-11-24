-- Undo 0003_multitenant: back to one shop with owner and cashier accounts.
-- The app never runs this file. Apply it by hand, with the app stopped:
--     psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -1 -f 0003_multitenant.down.sql
--
-- What a rollback can and cannot restore:
--   * It refuses to run when more than one shop has data. Two shops cannot share one catalog
--     (SKUs, barcodes and category names are unique again, and so are receipt numbers).
--     Export or remove the extra shops first.
--   * Each kasir seat becomes a cashier account (same ID, so its orders and stock movements stay
--     linked), named kasir-<id>. Their login is locked ('!'); reset it in the old app's Pengguna page.
--     Original cashier usernames and passwords are gone: the up migration did not keep them.
--   * Admin accounts become owner ("Admin") accounts, the only elevated role the old schema had.
--   * Orders sold by an owner point at that owner. Seat sales point at the seat's cashier account.
--   * Per-shop receipt numbers go back to one global counter, continuing after the highest number.
--   * Kasir seat limits (users.max_cashiers) and the receipt counter row are dropped.

-- ---------------------------------------------------------------- row-level security off first
-- With FORCE on, even this script would see no rows without app.tenant_id.
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['categories', 'products', 'orders', 'order_items', 'stock_movements', 'settings'] LOOP
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
        EXECUTE format('ALTER TABLE %I NO FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
    END LOOP;
END $$;
DROP FUNCTION IF EXISTS app_tenant();

-- ---------------------------------------------------------------- only one shop may have data
DO $$
DECLARE
    shops INTEGER;
    keep  UUID;
BEGIN
    SELECT count(DISTINCT d.t), min(d.t::text)::uuid INTO shops, keep
      FROM (SELECT tenant_id AS t FROM categories
            UNION ALL SELECT tenant_id FROM products
            UNION ALL SELECT tenant_id FROM orders) d;

    IF shops > 1 THEN
        RAISE EXCEPTION 'rollback refused: % shops have data, and the old schema holds exactly one', shops;
    END IF;

    -- The surviving settings row is the shop's that has data, or else the oldest owner's.
    IF keep IS NULL THEN
        SELECT id INTO keep FROM users WHERE role = 'owner' ORDER BY created_at, id LIMIT 1;
    END IF;
    DELETE FROM settings WHERE tenant_id IS DISTINCT FROM keep;
END $$;

-- ---------------------------------------------------------------- accounts
UPDATE users SET role = 'owner' WHERE role = 'admin';
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('owner', 'cashier'));

INSERT INTO users (id, username, display_name, password_hash, role, is_active, created_at)
SELECT id, 'kasir-' || replace(id::text, '-', ''), left(name, 80), '!', 'cashier', is_active, created_at
  FROM cashier_seats;

-- ---------------------------------------------------------------- orders: cashier_id comes back
ALTER TABLE orders ADD COLUMN cashier_id UUID;
UPDATE orders SET cashier_id = COALESCE(seat_id, tenant_id);
ALTER TABLE orders ALTER COLUMN cashier_id SET NOT NULL;
ALTER TABLE orders ADD CONSTRAINT orders_cashier_id_fkey
    FOREIGN KEY (cashier_id) REFERENCES users (id) ON DELETE RESTRICT;
CREATE INDEX orders_cashier_created_idx ON orders (cashier_id, created_at DESC);

-- ---------------------------------------------------------------- stock movements
UPDATE stock_movements SET user_id = seat_id WHERE user_id IS NULL; -- the seat's cashier account
ALTER TABLE stock_movements
    DROP CONSTRAINT IF EXISTS stock_movements_actor_check,
    DROP CONSTRAINT IF EXISTS stock_movements_tenant_seat_fkey,
    DROP CONSTRAINT IF EXISTS stock_movements_tenant_product_fkey;
ALTER TABLE stock_movements DROP COLUMN seat_id, DROP COLUMN tenant_id;
ALTER TABLE stock_movements ALTER COLUMN user_id SET NOT NULL;

-- ---------------------------------------------------------------- order items
ALTER TABLE order_items
    DROP CONSTRAINT IF EXISTS order_items_tenant_order_fkey,
    DROP CONSTRAINT IF EXISTS order_items_tenant_product_fkey;
ALTER TABLE order_items DROP COLUMN tenant_id;

-- ---------------------------------------------------------------- orders: global receipt numbers
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_tenant_seat_fkey;
DROP INDEX IF EXISTS orders_tenant_completed_created_idx;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_tenant_id_id_key;
DROP INDEX IF EXISTS orders_tenant_receipt_key;
ALTER TABLE orders ADD CONSTRAINT orders_receipt_no_key UNIQUE (receipt_no);
ALTER TABLE orders ALTER COLUMN receipt_no ADD GENERATED ALWAYS AS IDENTITY;
SELECT setval(pg_get_serial_sequence('orders', 'receipt_no'), COALESCE(max(receipt_no), 0) + 1, false) FROM orders;
ALTER TABLE orders DROP COLUMN seat_id, DROP COLUMN sold_by_name, DROP COLUMN tenant_id;
CREATE INDEX orders_completed_created_idx ON orders (created_at) WHERE status = 'completed';

-- ---------------------------------------------------------------- products and categories: global uniqueness
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_tenant_category_fkey;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_tenant_id_id_key;
DROP INDEX IF EXISTS products_tenant_sku_key;
DROP INDEX IF EXISTS products_tenant_barcode_key;
ALTER TABLE products DROP COLUMN tenant_id;
CREATE UNIQUE INDEX products_sku_key ON products (lower(sku));
CREATE UNIQUE INDEX products_barcode_key ON products (barcode) WHERE barcode IS NOT NULL;

ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_tenant_id_id_key;
DROP INDEX IF EXISTS categories_tenant_name_key;
ALTER TABLE categories DROP COLUMN tenant_id;
CREATE UNIQUE INDEX categories_name_key ON categories (lower(name));

-- ---------------------------------------------------------------- settings: back to the single id = 1 row
ALTER TABLE settings DROP CONSTRAINT IF EXISTS settings_pkey;
ALTER TABLE settings ADD COLUMN id SMALLINT NOT NULL DEFAULT 1;
ALTER TABLE settings DROP COLUMN tenant_id, DROP COLUMN last_receipt_no;
ALTER TABLE settings ADD PRIMARY KEY (id);
ALTER TABLE settings ADD CONSTRAINT settings_id_check CHECK (id = 1);
INSERT INTO settings (id) SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM settings);

-- ---------------------------------------------------------------- seats and limits go away
DROP TABLE cashier_seats;
ALTER TABLE users DROP COLUMN max_cashiers;

-- Forget that 0003 was applied, so the app applies it again on the next start if you roll forward.
DO $$
BEGIN
    IF to_regclass('schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
         WHERE version IN ('0003_multitenant', '0003_multitenant.sql', '0003_multitenant.up.sql');
    END IF;
END $$;
