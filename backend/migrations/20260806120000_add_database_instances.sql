-- +goose Up
-- +goose StatementBegin

CREATE TABLE database_instances (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID NOT NULL,
    name               TEXT NOT NULL,
    type               TEXT NOT NULL,
    host               TEXT NOT NULL,
    port               INT,
    username           TEXT NOT NULL,
    password           TEXT NOT NULL,
    ssl_mode           TEXT NOT NULL DEFAULT 'disable',
    ssl_client_cert    TEXT NOT NULL DEFAULT '',
    ssl_client_key     TEXT NOT NULL DEFAULT '',
    ssl_root_cert      TEXT NOT NULL DEFAULT '',
    is_tls_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    auth_database      TEXT NOT NULL DEFAULT 'admin',
    is_srv             BOOLEAN NOT NULL DEFAULT FALSE,
    last_discovered_at TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE database_instances
    ADD CONSTRAINT fk_database_instances_workspace_id
    FOREIGN KEY (workspace_id)
    REFERENCES workspaces (id)
    ON DELETE CASCADE;

CREATE INDEX idx_database_instances_workspace_id ON database_instances (workspace_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE database_instances;

-- +goose StatementEnd
