-- +goose Up
CREATE TABLE idempotency_record (
    key        TEXT        PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    program_id TEXT        NOT NULL,
    package_id BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_idem_created_at ON idempotency_record (created_at);

-- +goose Down
DROP TABLE IF EXISTS idempotency_record;