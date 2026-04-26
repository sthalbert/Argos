-- +goose Up
-- Virtual machines (ADR-0015) — non-Kubernetes platform infrastructure
-- inventoried from cloud-provider APIs. Mirrors the enriched nodes schema
-- where it makes sense, drops K8s-specific fields, adds cloud-native ones
-- unlocked by the rich cloud-provider payload (image, keypair, VPC, NICs,
-- security groups, block devices).
--
-- Soft-delete via terminated_at — rows are tombstoned, never hard-deleted,
-- so SecNumCloud audits can answer "what was running on date X?".
-- Reconciliation moves rows to terminated_at = NOW() when they disappear
-- from the live cloud-provider listing.
--
-- Resource values are kept as TEXT (cloud providers serialise them
-- inconsistently); parsing to bytes is a reader concern. NICs / SGs /
-- block_devices are JSONB so the collector forwards opaque structures
-- without forcing a schema for every provider's shape.

CREATE TABLE virtual_machines (
    id                      UUID PRIMARY KEY,
    cloud_account_id        UUID NOT NULL REFERENCES cloud_accounts(id) ON DELETE CASCADE,

    -- identity
    provider_vm_id          TEXT NOT NULL,
    name                    TEXT NOT NULL,
    display_name            TEXT,
    role                    TEXT,

    -- networking
    private_ip              TEXT,
    public_ip               TEXT,
    private_dns_name        TEXT,
    vpc_id                  TEXT,
    subnet_id               TEXT,
    nics                    JSONB NOT NULL DEFAULT '[]'::jsonb,
    security_groups         JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- cloud identity
    instance_type           TEXT,
    architecture            TEXT,
    zone                    TEXT,
    region                  TEXT,
    image_id                TEXT,
    keypair_name            TEXT,
    boot_mode               TEXT,
    provider_account_id     TEXT,
    provider_creation_date  TIMESTAMPTZ,

    -- power
    power_state             TEXT NOT NULL,
    state_reason            TEXT,
    ready                   BOOLEAN NOT NULL DEFAULT FALSE,
    deletion_protection     BOOLEAN NOT NULL DEFAULT FALSE,

    -- guest OS (nullable until/unless an in-guest agent is deployed)
    kernel_version          TEXT,
    operating_system        TEXT,

    -- capacity (parsed from instance_type when the family is recognised)
    capacity_cpu            TEXT,
    capacity_memory         TEXT,

    -- storage
    block_devices           JSONB NOT NULL DEFAULT '[]'::jsonb,
    root_device_type        TEXT,
    root_device_name        TEXT,

    -- semi-structured
    tags                    JSONB NOT NULL DEFAULT '{}'::jsonb,
    labels                  JSONB NOT NULL DEFAULT '{}'::jsonb,
    annotations             JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- curated
    owner                   TEXT,
    criticality             TEXT,
    notes                   TEXT,
    runbook_url             TEXT,

    -- lifecycle (soft-delete tombstone)
    created_at              TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL,
    last_seen_at            TIMESTAMPTZ NOT NULL,
    terminated_at           TIMESTAMPTZ,

    UNIQUE (cloud_account_id, provider_vm_id)
);

CREATE INDEX virtual_machines_cloud_account_id_idx ON virtual_machines (cloud_account_id);
CREATE INDEX virtual_machines_terminated_at_idx ON virtual_machines (terminated_at);
CREATE INDEX virtual_machines_role_idx ON virtual_machines (role);
CREATE INDEX virtual_machines_power_state_idx ON virtual_machines (power_state);
CREATE INDEX virtual_machines_created_at_id_idx ON virtual_machines (created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS virtual_machines_created_at_id_idx;
DROP INDEX IF EXISTS virtual_machines_power_state_idx;
DROP INDEX IF EXISTS virtual_machines_role_idx;
DROP INDEX IF EXISTS virtual_machines_terminated_at_idx;
DROP INDEX IF EXISTS virtual_machines_cloud_account_id_idx;
DROP TABLE IF EXISTS virtual_machines;
