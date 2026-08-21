-- +goose Up
CREATE TABLE IF NOT EXISTS data_export_artifacts (
    id varchar(36) PRIMARY KEY,
    organization_id varchar(36) NOT NULL,
    actor_id varchar(36) NOT NULL,
    approval_id varchar(36) NOT NULL,
    object_key varchar(512) NOT NULL,
    content_type varchar(200) NOT NULL,
    sha256 char(64) NOT NULL,
    size_bytes bigint NOT NULL,
    status varchar(40) NOT NULL,
    max_downloads integer NOT NULL,
    downloads integer NOT NULL DEFAULT 0,
    expires_at datetime(6) NOT NULL,
    created_at datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    downloaded_at datetime(6),
    revoked_at datetime(6),
    revoked_reason varchar(500),
    CONSTRAINT chk_data_export_artifacts_size CHECK (size_bytes > 0),
    CONSTRAINT chk_data_export_artifacts_downloads CHECK (downloads >= 0 AND downloads <= max_downloads),
    CONSTRAINT chk_data_export_artifacts_max_downloads CHECK (max_downloads > 0),
    CONSTRAINT fk_data_export_artifacts_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX idx_data_export_artifacts_org_status_expiry ON data_export_artifacts(organization_id, status, expires_at);

-- +goose Down
DROP TABLE IF EXISTS data_export_artifacts;
