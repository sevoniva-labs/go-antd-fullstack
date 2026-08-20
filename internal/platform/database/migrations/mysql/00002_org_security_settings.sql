-- +goose Up
ALTER TABLE organizations ADD COLUMN status varchar(20) NOT NULL DEFAULT 'ACTIVE';
ALTER TABLE organizations ADD COLUMN description varchar(500) NOT NULL DEFAULT '';
ALTER TABLE organizations ADD COLUMN max_users int NOT NULL DEFAULT 0;
ALTER TABLE organizations ADD COLUMN max_active_sessions int NOT NULL DEFAULT 0;

UPDATE organizations
  SET status = COALESCE(NULLIF(TRIM(status), ''), 'ACTIVE'),
      description = COALESCE(description, ''),
      max_users = COALESCE(max_users, 0),
      max_active_sessions = COALESCE(max_active_sessions, 0);

CREATE TABLE IF NOT EXISTS system_settings (
  organization_id varchar(36) NOT NULL,
  setting_key varchar(160) NOT NULL,
  setting_value text NOT NULL,
  updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_by varchar(120) NOT NULL DEFAULT '',
  PRIMARY KEY (organization_id, setting_key),
  KEY idx_system_settings_org (organization_id),
  KEY idx_system_settings_updated_at (updated_at),
  CONSTRAINT fk_system_settings_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS system_settings;
ALTER TABLE organizations DROP COLUMN status;
ALTER TABLE organizations DROP COLUMN description;
ALTER TABLE organizations DROP COLUMN max_users;
ALTER TABLE organizations DROP COLUMN max_active_sessions;
