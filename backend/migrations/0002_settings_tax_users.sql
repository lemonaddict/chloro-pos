-- Business profile (printed on receipts), tax, QRIS, and revocable sessions.

-- One row, always id = 1.
CREATE TABLE settings (
    id             SMALLINT    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    business_name  TEXT        NOT NULL DEFAULT 'Usaha Saya' CHECK (char_length(business_name) BETWEEN 1 AND 80),
    legal_name     TEXT        NOT NULL DEFAULT '' CHECK (char_length(legal_name) <= 120),
    address        TEXT        NOT NULL DEFAULT '' CHECK (char_length(address) <= 300),
    phone          TEXT        NOT NULL DEFAULT '' CHECK (char_length(phone) <= 30),
    email          TEXT        NOT NULL DEFAULT '' CHECK (char_length(email) <= 120),
    npwp           TEXT        NOT NULL DEFAULT '' CHECK (npwp ~ '^([0-9]{15}|[0-9]{16})?$'),
    receipt_footer TEXT        NOT NULL DEFAULT 'Terima kasih sudah berbelanja!' CHECK (char_length(receipt_footer) <= 200),
    tax_enabled    BOOLEAN     NOT NULL DEFAULT FALSE,
    tax_label      TEXT        NOT NULL DEFAULT 'PPN' CHECK (char_length(tax_label) BETWEEN 1 AND 20),
    tax_rate_bps   INTEGER     NOT NULL DEFAULT 1100 CHECK (tax_rate_bps BETWEEN 0 AND 10000), -- 1100 = 11%
    tax_inclusive  BOOLEAN     NOT NULL DEFAULT TRUE,   -- prices already include tax
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO settings (id) VALUES (1);

-- Each order keeps the tax rules it was sold under, so old receipts never change.
ALTER TABLE orders
    ADD COLUMN subtotal_cents BIGINT,
    ADD COLUMN tax_cents      BIGINT  NOT NULL DEFAULT 0 CHECK (tax_cents >= 0),
    ADD COLUMN tax_rate_bps   INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN tax_inclusive  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN tax_label      TEXT    NOT NULL DEFAULT '';
UPDATE orders SET subtotal_cents = total_cents;
ALTER TABLE orders
    ALTER COLUMN subtotal_cents SET NOT NULL,
    ADD CONSTRAINT orders_subtotal_check CHECK (subtotal_cents >= 0),
    ADD CONSTRAINT orders_total_matches_check
        CHECK (total_cents = subtotal_cents + CASE WHEN tax_inclusive THEN 0 ELSE tax_cents END);

-- QRIS joins the payment methods.
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_payment_method_check;
ALTER TABLE orders ADD CONSTRAINT orders_payment_method_check
    CHECK (payment_method IN ('cash', 'card', 'transfer', 'qris'));

-- Sessions issued before this moment are rejected. Set on deactivation and password reset.
ALTER TABLE users ADD COLUMN sessions_valid_after TIMESTAMPTZ NOT NULL DEFAULT now();
