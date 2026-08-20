-- +goose Up
CREATE TABLE IF NOT EXISTS emergency_access_grants (
  id varchar(36) PRIMARY KEY,
  organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
  requester_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  target_user_id varchar(36) NULL REFERENCES users(id) ON DELETE RESTRICT,
  scope varchar(200) NOT NULL,
  approval_id varchar(36) NOT NULL UNIQUE REFERENCES approval_requests(id) ON DELETE RESTRICT,
  reason varchar(500) NOT NULL,
  privilege_keys_json text NOT NULL,
  requested_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz NULL,
  revoked_by varchar(36) NULL REFERENCES users(id) ON DELETE RESTRICT,
  revoke_reason varchar(500) NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT ck_emergency_access_validity CHECK (expires_at > requested_at)
);
CREATE INDEX IF NOT EXISTS idx_emergency_access_effective ON emergency_access_grants(organization_id,expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_emergency_access_created ON emergency_access_grants(organization_id,created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS emergency_access_grants;
