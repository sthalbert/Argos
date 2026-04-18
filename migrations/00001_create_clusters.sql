-- +goose Up
CREATE TABLE clusters (
    id                 UUID PRIMARY KEY,
    name               TEXT NOT NULL UNIQUE,
    display_name       TEXT,
    environment        TEXT,
    provider           TEXT,
    region             TEXT,
    kubernetes_version TEXT,
    api_endpoint       TEXT,
    labels             JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL
);

CREATE INDEX clusters_created_at_id_idx ON clusters (created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS clusters_created_at_id_idx;
DROP TABLE IF EXISTS clusters;
