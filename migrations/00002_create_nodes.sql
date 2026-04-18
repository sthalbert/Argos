-- +goose Up
CREATE TABLE nodes (
    id               UUID PRIMARY KEY,
    cluster_id       UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    display_name     TEXT,
    kubelet_version  TEXT,
    os_image         TEXT,
    architecture     TEXT,
    labels           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL,
    UNIQUE (cluster_id, name)
);

CREATE INDEX nodes_cluster_id_idx ON nodes (cluster_id);
CREATE INDEX nodes_created_at_id_idx ON nodes (created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS nodes_created_at_id_idx;
DROP INDEX IF EXISTS nodes_cluster_id_idx;
DROP TABLE IF EXISTS nodes;
