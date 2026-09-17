-- +goose Up
-- +goose StatementBegin
-- attempt_count carried two meanings: the retry counter the backoff reads and the
-- generation a write receipt matches. Every deletion request raised it, so a file
-- requested a few times began its first real attempt already deep in the backoff.
-- Generation moves on its own now, and attempt_count counts only attempts.
ALTER TABLE storage_pending_deletions
    ADD COLUMN generation INTEGER NOT NULL DEFAULT 0;

UPDATE storage_pending_deletions
SET generation = attempt_count;

ALTER TABLE storage_pending_deletions
    ADD CONSTRAINT ck_storage_pending_deletions_generation_not_negative CHECK (generation >= 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE storage_pending_deletions
    DROP CONSTRAINT IF EXISTS ck_storage_pending_deletions_generation_not_negative;

ALTER TABLE storage_pending_deletions
    DROP COLUMN IF EXISTS generation;
-- +goose StatementEnd
