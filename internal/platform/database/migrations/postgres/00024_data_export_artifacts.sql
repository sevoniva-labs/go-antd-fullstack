-- +goose Up
CREATE TABLE IF NOT EXISTS data_export_artifacts (
    id varchar(36) PRIMARY KEY,
    organization_id varchar(36) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_id varchar(36) NOT NULL,
    approval_id varchar(36) NOT NULL,
    object_key varchar(512) NOT NULL,
    content_type varchar(200) NOT NULL,
    sha256 char(64) NOT NULL,
    size_bytes bigint NOT NULL,
    status varchar(40) NOT NULL,
    max_downloads integer NOT NULL,
    downloads integer NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    downloaded_at timestamptz,
    revoked_at timestamptz,
    revoked_reason varchar(500),
    CONSTRAINT chk_data_export_artifacts_size CHECK (size_bytes > 0),
    CONSTRAINT chk_data_export_artifacts_downloads CHECK (downloads >= 0 AND downloads <= max_downloads),
    CONSTRAINT chk_data_export_artifacts_max_downloads CHECK (max_downloads > 0)
);
CREATE INDEX IF NOT EXISTS idx_data_export_artifacts_org_status_expiry ON data_export_artifacts(organization_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_data_export_artifacts_status_expiry ON data_export_artifacts(status, expires_at, id);
CREATE INDEX IF NOT EXISTS idx_data_export_artifacts_status_updated ON data_export_artifacts(status, updated_at, id);

-- +goose Down
DROP TABLE IF EXISTS data_export_artifacts;
