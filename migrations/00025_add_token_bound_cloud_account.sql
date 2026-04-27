-- +goose Up
-- Token-to-cloud-account binding (ADR-0015). Tokens carrying the
-- vm-collector scope are issued tied to one specific cloud_account so
-- a leaked PAT cannot be used against a different account's resources.
-- Nullable for every other token kind (admin/editor/auditor/viewer
-- tokens are not bound).
--
-- ON DELETE CASCADE: deleting a cloud_account invalidates its bound
-- tokens — they have nothing left to authorise. The admin operator who
-- removes the account is also (logically) revoking its tokens.

ALTER TABLE api_tokens
    ADD COLUMN bound_cloud_account_id UUID REFERENCES cloud_accounts(id) ON DELETE CASCADE;

CREATE INDEX api_tokens_bound_cloud_account_idx ON api_tokens (bound_cloud_account_id)
    WHERE bound_cloud_account_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS api_tokens_bound_cloud_account_idx;
ALTER TABLE api_tokens DROP COLUMN IF EXISTS bound_cloud_account_id;
