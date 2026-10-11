-- +goose Up
CREATE TABLE gift_package (
    program_id     TEXT        NOT NULL,
    package_id     BIGINT      NOT NULL,
    name           TEXT        NOT NULL,
    weight         INT         NOT NULL,
    total_quantity INT         NOT NULL,
    win_quantity   INT         NOT NULL DEFAULT 0,
    max_per_user   INT         NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (program_id, package_id),
    CONSTRAINT chk_win_le_total  CHECK (win_quantity <= total_quantity),
    CONSTRAINT chk_win_nonneg    CHECK (win_quantity >= 0),
    CONSTRAINT chk_weight_pos    CHECK (weight > 0)
);

CREATE TABLE user_quota (
    user_id    BIGINT      NOT NULL,
    program_id TEXT        NOT NULL,
    package_id BIGINT      NOT NULL,
    used       INT         NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, program_id, package_id),
    CONSTRAINT chk_used_nonneg CHECK (used >= 0)
);

CREATE TABLE draw_log (
    log_id     BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    program_id TEXT        NOT NULL,
    package_id BIGINT      NOT NULL,
    status     TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);



CREATE INDEX idx_draw_log_user_program ON draw_log (user_id, program_id);




-- +goose Down
DROP TABLE IF EXISTS draw_log;

DROP TABLE IF EXISTS user_quota;
DROP TABLE IF EXISTS gift_package;
