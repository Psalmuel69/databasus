-- +goose Up
-- +goose StatementBegin
-- A row here is an outstanding obligation to remove one stored file. It is created
-- before the first byte reaches the provider and removed by the transaction that
-- publishes the file as part of a backup, so every other outcome (provider error,
-- crash, rolled back catalog write, cancellation) leaves the obligation standing.
CREATE TABLE storage_pending_deletions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    storage_id    UUID        NOT NULL,
    file_name     TEXT        NOT NULL,
    -- Earliest moment the cleanup worker may act on this file. A fresh write starts
    -- one commit window in the future; a failed write and every deletion request
    -- move it to now.
    not_before    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Two meanings on purpose. It counts deletion attempts for the retry backoff,
    -- and it is the generation a write receipt carries: anything that takes the file
    -- away from its writer increments it, so the stale receipt can no longer publish.
    attempt_count INTEGER     NOT NULL DEFAULT 0,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE storage_pending_deletions
    ADD CONSTRAINT fk_storage_pending_deletions_storage_id
        FOREIGN KEY (storage_id) REFERENCES storages (id) ON DELETE CASCADE;

ALTER TABLE storage_pending_deletions
    ADD CONSTRAINT uq_storage_pending_deletions_storage_file UNIQUE (storage_id, file_name);

ALTER TABLE storage_pending_deletions
    ADD CONSTRAINT ck_storage_pending_deletions_file_name_not_blank CHECK (file_name <> '');

ALTER TABLE storage_pending_deletions
    ADD CONSTRAINT ck_storage_pending_deletions_attempt_count_not_negative CHECK (attempt_count >= 0);

CREATE INDEX idx_storage_pending_deletions_not_before
    ON storage_pending_deletions (not_before);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_storage_pending_deletions_not_before;

ALTER TABLE storage_pending_deletions
    DROP CONSTRAINT IF EXISTS ck_storage_pending_deletions_attempt_count_not_negative;

ALTER TABLE storage_pending_deletions
    DROP CONSTRAINT IF EXISTS ck_storage_pending_deletions_file_name_not_blank;

ALTER TABLE storage_pending_deletions
    DROP CONSTRAINT IF EXISTS uq_storage_pending_deletions_storage_file;

ALTER TABLE storage_pending_deletions
    DROP CONSTRAINT IF EXISTS fk_storage_pending_deletions_storage_id;

DROP TABLE IF EXISTS storage_pending_deletions;
-- +goose StatementEnd
