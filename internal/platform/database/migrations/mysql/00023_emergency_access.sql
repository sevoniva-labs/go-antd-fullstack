-- +goose Up
CREATE TABLE IF NOT EXISTS emergency_access_grants (
  id varchar(36) NOT NULL PRIMARY KEY,
  organization_id varchar(36) NOT NULL,
  requester_id varchar(36) NOT NULL,
  target_user_id varchar(36) NULL,
  scope varchar(200) NOT NULL,
  approval_id varchar(36) NOT NULL,
  reason varchar(500) NOT NULL,
  privilege_keys_json text NOT NULL,
  requested_at timestamp(6) NOT NULL,
  expires_at timestamp(6) NOT NULL,
  revoked_at timestamp(6) NULL,
  revoked_by varchar(36) NULL,
  revoke_reason varchar(500) NULL,
  created_at timestamp(6) NOT NULL,
  UNIQUE KEY uk_emergency_access_approval (approval_id),
  KEY idx_emergency_access_effective (organization_id,revoked_at,expires_at),
  KEY idx_emergency_access_created (organization_id,created_at),
  CONSTRAINT fk_emergency_access_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT,
  CONSTRAINT fk_emergency_access_requester FOREIGN KEY (requester_id) REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT fk_emergency_access_target FOREIGN KEY (target_user_id) REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT fk_emergency_access_approval FOREIGN KEY (approval_id) REFERENCES approval_requests(id) ON DELETE RESTRICT,
  CONSTRAINT fk_emergency_access_revoker FOREIGN KEY (revoked_by) REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT ck_emergency_access_validity CHECK (expires_at > requested_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS emergency_access_grants;
