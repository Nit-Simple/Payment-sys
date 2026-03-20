-- migrations/002_idempotency_status.sql

ALTER TABLE idempotency_keys
ADD COLUMN status TEXT NOT NULL DEFAULT 'pending';

ALTER TABLE idempotency_keys
ALTER COLUMN response_status DROP NOT NULL;