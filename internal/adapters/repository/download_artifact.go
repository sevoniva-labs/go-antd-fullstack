package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/sevoniva-labs/forge/internal/platform/database"
	securitypolicy "github.com/sevoniva-labs/forge/internal/platform/security/datapolicy"
)

// DownloadArtifactRepo persists the one-time export ticket. ClaimDownload
// locks the row and performs the state transition in the same transaction so
// multiple worker/API instances cannot both receive the bytes.
type DownloadArtifactRepo struct{ db *database.DB }

func NewDownloadArtifactRepo(db *database.DB) *DownloadArtifactRepo {
	return &DownloadArtifactRepo{db: db}
}

func (r *DownloadArtifactRepo) Create(ctx context.Context, artifact securitypolicy.DownloadArtifact) error {
	if err := validateDownloadArtifact(artifact); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO data_export_artifacts(id,organization_id,actor_id,approval_id,object_key,content_type,sha256,size_bytes,status,max_downloads,downloads,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), artifact.ID, artifact.OrganizationID, artifact.ActorID, artifact.ApprovalID, artifact.ObjectKey, artifact.ContentType, artifact.SHA256, artifact.Size, artifact.Status, artifact.MaxDownloads, artifact.Downloads, artifact.ExpiresAt, artifact.CreatedAt, artifact.UpdatedAt)
	return err
}

func (r *DownloadArtifactRepo) Get(ctx context.Context, organizationID, artifactID string) (securitypolicy.DownloadArtifact, error) {
	return r.scan(r.db.QueryRowContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE organization_id=? AND id=?`), organizationID, artifactID))
}

func (r *DownloadArtifactRepo) ClaimDownload(ctx context.Context, organizationID, artifactID, actorID string, now time.Time) (securitypolicy.DownloadArtifact, error) {
	var result securitypolicy.DownloadArtifact
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		artifact, err := r.scan(tx.QueryRowContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE organization_id=? AND id=? FOR UPDATE`), organizationID, artifactID))
		if err != nil {
			return err
		}
		if artifact.ActorID != strings.TrimSpace(actorID) {
			return errors.New("download actor is not authorized")
		}
		if err := validateClaimable(artifact, now); err != nil {
			if errors.Is(err, securitypolicy.ErrDownloadArtifactExpired) {
				_, _ = tx.ExecContext(ctx, r.db.Rebind(`UPDATE data_export_artifacts SET status=?,updated_at=? WHERE organization_id=? AND id=? AND status=?`), securitypolicy.DownloadArtifactExpired, now.UTC(), organizationID, artifactID, securitypolicy.DownloadArtifactReady)
			}
			return err
		}
		result, err = r.scan(tx.QueryRowContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE organization_id=? AND id=?`), organizationID, artifactID))
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE data_export_artifacts SET status=?,downloads=downloads+1,downloaded_at=?,updated_at=? WHERE organization_id=? AND id=? AND status=? AND downloads<max_downloads AND expires_at>?`), securitypolicy.DownloadArtifactDownloaded, now.UTC(), now.UTC(), organizationID, artifactID, securitypolicy.DownloadArtifactReady, now.UTC())
		if err != nil {
			return err
		}
		result, err = r.scan(tx.QueryRowContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE organization_id=? AND id=?`), organizationID, artifactID))
		return err
	})
	return result, err
}

func (r *DownloadArtifactRepo) Revoke(ctx context.Context, organizationID, artifactID, actorID, reason string, now time.Time) (securitypolicy.DownloadArtifact, error) {
	var result securitypolicy.DownloadArtifact
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		artifact, err := r.scan(tx.QueryRowContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE organization_id=? AND id=? FOR UPDATE`), organizationID, artifactID))
		if err != nil {
			return err
		}
		if artifact.ActorID != strings.TrimSpace(actorID) {
			return errors.New("download actor is not authorized")
		}
		if artifact.Status != securitypolicy.DownloadArtifactReady {
			return securitypolicy.ErrDownloadArtifactConsumed
		}
		if strings.TrimSpace(reason) == "" || len(reason) > 500 {
			return securitypolicy.ErrDownloadArtifactInvalid
		}
		_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE data_export_artifacts SET status=?,revoked_at=?,revoked_reason=?,updated_at=? WHERE organization_id=? AND id=? AND status=?`), securitypolicy.DownloadArtifactRevoked, now.UTC(), strings.TrimSpace(reason), now.UTC(), organizationID, artifactID, securitypolicy.DownloadArtifactReady)
		if err != nil {
			return err
		}
		result, err = r.scan(tx.QueryRowContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE organization_id=? AND id=?`), organizationID, artifactID))
		return err
	})
	return result, err
}

func (r *DownloadArtifactRepo) Expire(ctx context.Context, organizationID, artifactID string, now time.Time) (securitypolicy.DownloadArtifact, error) {
	var result securitypolicy.DownloadArtifact
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		artifact, err := r.scan(tx.QueryRowContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE organization_id=? AND id=? FOR UPDATE`), organizationID, artifactID))
		if err != nil {
			return err
		}
		if artifact.Status != securitypolicy.DownloadArtifactReady {
			return securitypolicy.ErrDownloadArtifactConsumed
		}
		if now.Before(artifact.ExpiresAt) {
			return errors.New("download artifact has not expired")
		}
		_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE data_export_artifacts SET status=?,updated_at=? WHERE organization_id=? AND id=? AND status=?`), securitypolicy.DownloadArtifactExpired, now.UTC(), organizationID, artifactID, securitypolicy.DownloadArtifactReady)
		if err != nil {
			return err
		}
		result, err = r.scan(tx.QueryRowContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE organization_id=? AND id=?`), organizationID, artifactID))
		return err
	})
	return result, err
}

func (r *DownloadArtifactRepo) MarkCleanupPending(ctx context.Context, organizationID, artifactID string, now time.Time) (securitypolicy.DownloadArtifact, error) {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE data_export_artifacts SET status=?,updated_at=? WHERE organization_id=? AND id=? AND status IN (?,?,?)`), securitypolicy.DownloadArtifactCleanup, now.UTC(), organizationID, artifactID, securitypolicy.DownloadArtifactRevoked, securitypolicy.DownloadArtifactExpired, securitypolicy.DownloadArtifactDownloaded)
	if err != nil {
		return securitypolicy.DownloadArtifact{}, err
	}
	return r.Get(ctx, organizationID, artifactID)
}

func (r *DownloadArtifactRepo) CompleteCleanup(ctx context.Context, organizationID, artifactID string, now time.Time) (securitypolicy.DownloadArtifact, error) {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE data_export_artifacts SET status=CASE WHEN revoked_at IS NOT NULL THEN ? WHEN downloaded_at IS NOT NULL THEN ? ELSE ? END,updated_at=? WHERE organization_id=? AND id=? AND status=?`), securitypolicy.DownloadArtifactRevoked, securitypolicy.DownloadArtifactDownloaded, securitypolicy.DownloadArtifactExpired, now.UTC(), organizationID, artifactID, securitypolicy.DownloadArtifactCleanup)
	if err != nil {
		return securitypolicy.DownloadArtifact{}, err
	}
	return r.Get(ctx, organizationID, artifactID)
}

func (r *DownloadArtifactRepo) ListCleanupPending(ctx context.Context, limit int) ([]securitypolicy.DownloadArtifact, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE status=? ORDER BY updated_at,id LIMIT ?`), securitypolicy.DownloadArtifactCleanup, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]securitypolicy.DownloadArtifact, 0, limit)
	for rows.Next() {
		item, scanErr := r.scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *DownloadArtifactRepo) ListExpiredReady(ctx context.Context, now time.Time, limit int) ([]securitypolicy.DownloadArtifact, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(downloadArtifactSelect+` WHERE status=? AND expires_at<=? ORDER BY expires_at,id LIMIT ?`), securitypolicy.DownloadArtifactReady, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]securitypolicy.DownloadArtifact, 0, limit)
	for rows.Next() {
		item, scanErr := r.scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const downloadArtifactSelect = `SELECT id,organization_id,actor_id,approval_id,object_key,content_type,sha256,size_bytes,status,max_downloads,downloads,expires_at,created_at,updated_at,downloaded_at,revoked_at,revoked_reason FROM data_export_artifacts`

type downloadArtifactScanner interface{ Scan(...any) error }

func (r *DownloadArtifactRepo) scan(scanner downloadArtifactScanner) (securitypolicy.DownloadArtifact, error) {
	var artifact securitypolicy.DownloadArtifact
	var downloadedAt, revokedAt sql.NullTime
	var revokedReason sql.NullString
	if err := scanner.Scan(&artifact.ID, &artifact.OrganizationID, &artifact.ActorID, &artifact.ApprovalID, &artifact.ObjectKey, &artifact.ContentType, &artifact.SHA256, &artifact.Size, &artifact.Status, &artifact.MaxDownloads, &artifact.Downloads, &artifact.ExpiresAt, &artifact.CreatedAt, &artifact.UpdatedAt, &downloadedAt, &revokedAt, &revokedReason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return securitypolicy.DownloadArtifact{}, securitypolicy.ErrDownloadArtifactNotFound
		}
		return securitypolicy.DownloadArtifact{}, err
	}
	if downloadedAt.Valid {
		artifact.DownloadedAt = downloadedAt.Time
	}
	if revokedAt.Valid {
		artifact.RevokedAt = revokedAt.Time
	}
	if revokedReason.Valid {
		artifact.RevokedReason = revokedReason.String
	}
	return artifact, nil
}

func validateDownloadArtifact(artifact securitypolicy.DownloadArtifact) error {
	if artifact.ID == "" || artifact.OrganizationID == "" || artifact.ActorID == "" || artifact.ApprovalID == "" || artifact.ObjectKey == "" || artifact.ContentType == "" || len(artifact.SHA256) != 64 || artifact.Size <= 0 || artifact.Status != securitypolicy.DownloadArtifactReady || artifact.MaxDownloads != 1 || artifact.Downloads != 0 || artifact.ExpiresAt.IsZero() || artifact.CreatedAt.IsZero() || artifact.UpdatedAt.IsZero() {
		return securitypolicy.ErrDownloadArtifactInvalid
	}
	return nil
}

func validateClaimable(artifact securitypolicy.DownloadArtifact, now time.Time) error {
	switch artifact.Status {
	case securitypolicy.DownloadArtifactRevoked:
		return securitypolicy.ErrDownloadArtifactRevoked
	case securitypolicy.DownloadArtifactDownloaded:
		return securitypolicy.ErrDownloadArtifactConsumed
	case securitypolicy.DownloadArtifactReady:
		if artifact.Downloads >= artifact.MaxDownloads {
			return securitypolicy.ErrDownloadArtifactConsumed
		}
		if now.UTC().After(artifact.ExpiresAt) || now.UTC().Equal(artifact.ExpiresAt) {
			return securitypolicy.ErrDownloadArtifactExpired
		}
		return nil
	default:
		return securitypolicy.ErrDownloadArtifactInvalid
	}
}
