-- +goose Up
-- Running / declared containers for Pods and Workloads. Flat JSONB array;
-- each entry typically carries {name, image, image_id, init} but the shape
-- is open (collector may add fields). Defaults to '[]' so existing rows
-- backfill cleanly without a separate UPDATE.
ALTER TABLE pods      ADD COLUMN containers JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE workloads ADD COLUMN containers JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE workloads DROP COLUMN IF EXISTS containers;
ALTER TABLE pods      DROP COLUMN IF EXISTS containers;
