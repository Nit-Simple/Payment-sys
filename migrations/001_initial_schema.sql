-- migrations/001_initial_schema.sql

-- enums
CREATE TYPE payment_status AS ENUM (
    'initiated',
    'processing',
    'succeeded',
    'failed',
    'cancelled',
    'refunded'
);

CREATE TYPE payment_method AS ENUM (
    'card',
    'upi',
    'bank'
);

-- payments
CREATE TABLE payments (
    id                  UUID PRIMARY KEY,
    customer_id         TEXT NOT NULL,
    amount              BIGINT NOT NULL,
    currency            CHAR(3) NOT NULL,
    status              payment_status NOT NULL DEFAULT 'initiated',
    method              payment_method NOT NULL,

    -- card fields
    card_last_four      TEXT,
    card_brand          TEXT,
    card_fingerprint    TEXT,
    encrypted_card_data BYTEA,

    -- upi fields
    upi_id              TEXT,

    -- bank fields
    account_number      TEXT,
    ifsc_code           TEXT,
    account_holder_name TEXT,

    -- common
    email               TEXT NOT NULL,
    ip_address          INET,
    metadata            JSONB,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- constraints
    CONSTRAINT amount_positive CHECK (amount > 0),
    CONSTRAINT currency_length CHECK (char_length(currency) = 3),

    -- card fields must be present together or not at all
    CONSTRAINT card_fields CHECK (
        (method = 'card' AND card_last_four IS NOT NULL AND card_brand IS NOT NULL AND card_fingerprint IS NOT NULL AND encrypted_card_data IS NOT NULL)
        OR (method != 'card' AND card_last_four IS NULL AND card_brand IS NULL AND card_fingerprint IS NULL AND encrypted_card_data IS NULL)
    ),

    -- upi field must be present only for upi method
    CONSTRAINT upi_fields CHECK (
        (method = 'upi' AND upi_id IS NOT NULL)
        OR (method != 'upi' AND upi_id IS NULL)
    ),

    -- bank fields must be present together or not at all
    CONSTRAINT bank_fields CHECK (
        (method = 'bank' AND account_number IS NOT NULL AND ifsc_code IS NOT NULL AND account_holder_name IS NOT NULL)
        OR (method != 'bank' AND account_number IS NULL AND ifsc_code IS NULL AND account_holder_name IS NULL)
    )
);

-- payment_events
CREATE TABLE payment_events (
    id          UUID PRIMARY KEY ,
    payment_id  UUID NOT NULL REFERENCES payments(id),
    from_status payment_status,
    to_status   payment_status NOT NULL,
    reason      TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- idempotency_keys
CREATE TABLE idempotency_keys (
    key             TEXT PRIMARY KEY,
    payment_id      UUID REFERENCES payments(id),
    request_hash    TEXT NOT NULL,
    response_status INTEGER NOT NULL,
    response_body   JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL
);

-- updated_at trigger
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- indexes
CREATE INDEX idx_payments_customer_id    ON payments(customer_id);
CREATE INDEX idx_payments_status         ON payments(status);
CREATE INDEX idx_payments_created_at     ON payments(created_at);
CREATE INDEX idx_payments_card_fingerprint ON payments(card_fingerprint) WHERE card_fingerprint IS NOT NULL;

CREATE INDEX idx_payment_events_payment_id ON payment_events(payment_id);

CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys(expires_at);