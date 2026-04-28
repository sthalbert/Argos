-- +goose Up
-- ADR-0016: differentiate audit rows by ingress path. Requests entering the
-- trusted zone via the new mTLS-only ingest listener (collectors behind the
-- DMZ ingest gateway) are tagged 'ingest_gw'; everything else stays 'api'.
-- Operators tracing "what came through the DMZ" can answer it with a single
-- WHERE clause instead of cross-referencing source IPs.
ALTER TABLE audit_events
    ADD COLUMN source TEXT NOT NULL DEFAULT 'api';

ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_source_check
    CHECK (source IN ('api', 'ingest_gw', 'system'));

-- Existing rows keep the default 'api' applied at column-add time. The
-- column is NOT NULL so future inserts must specify a source explicitly
-- (the audit middleware sets it from a per-listener label).

-- "Show me everything that came through the DMZ" is a frequent SNC audit
-- question; a small partial index keeps that query fast without bloating
-- the table for the typical case.
CREATE INDEX audit_events_source_ingest_idx ON audit_events (occurred_at DESC)
    WHERE source = 'ingest_gw';

-- +goose Down
DROP INDEX IF EXISTS audit_events_source_ingest_idx;
ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS audit_events_source_check;
ALTER TABLE audit_events DROP COLUMN IF EXISTS source;
