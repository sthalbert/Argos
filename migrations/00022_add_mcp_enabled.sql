-- +goose Up
ALTER TABLE settings ADD COLUMN mcp_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE settings DROP COLUMN mcp_enabled;
