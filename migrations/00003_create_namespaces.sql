-- +goose Up
CREATE TABLE namespaces (
    id            UUID PRIMARY KEY,
    cluster_id    UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    display_name  TEXT,
    phase         TEXT,
    labels        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL,
    UNIQUE (cluster_id, name)
);

CREATE INDEX namespaces_cluster_id_idx ON namespaces (cluster_id);
CREATE INDEX namespaces_created_at_id_idx ON namespaces (created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS namespaces_created_at_id_idx;
DROP INDEX IF EXISTS namespaces_cluster_id_idx;
DROP TABLE IF EXISTS namespaces;
