-- +goose Up
CREATE TABLE ingresses (
    id                  UUID PRIMARY KEY,
    namespace_id        UUID NOT NULL REFERENCES namespaces(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    ingress_class_name  TEXT,
    rules               JSONB NOT NULL DEFAULT '[]'::jsonb,
    tls                 JSONB NOT NULL DEFAULT '[]'::jsonb,
    labels              JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    UNIQUE (namespace_id, name)
);

CREATE INDEX ingresses_namespace_id_idx ON ingresses (namespace_id);
CREATE INDEX ingresses_created_at_id_idx ON ingresses (created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS ingresses_created_at_id_idx;
DROP INDEX IF EXISTS ingresses_namespace_id_idx;
DROP TABLE IF EXISTS ingresses;
