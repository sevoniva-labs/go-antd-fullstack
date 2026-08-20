package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
)

func (r *IdentityRepo) CreateEmergencyAccess(ctx context.Context, grant domain.EmergencyAccessGrant) (domain.EmergencyAccessGrant, error) {
	keys, err := json.Marshal(grant.PrivilegeKeys)
	if err != nil {
		return domain.EmergencyAccessGrant{}, fmt.Errorf("encode emergency access privileges: %w", err)
	}
	grant.ID = uuid.NewString()
	grant.CreatedAt = time.Now().UTC()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO emergency_access_grants(id,organization_id,requester_id,target_user_id,scope,approval_id,reason,privilege_keys_json,requested_at,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`), grant.ID, grant.OrganizationID, grant.RequesterID, nullIfEmpty(grant.TargetUserID), grant.Scope, grant.ApprovalID, grant.Reason, string(keys), grant.RequestedAt, grant.ExpiresAt, grant.CreatedAt)
	return grant, err
}

func (r *IdentityRepo) ListEmergencyAccess(ctx context.Context, organizationID string, limit int) ([]domain.EmergencyAccessGrant, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,requester_id,target_user_id,scope,approval_id,reason,privilege_keys_json,requested_at,expires_at,revoked_at,revoked_by,revoke_reason,created_at FROM emergency_access_grants WHERE organization_id=? ORDER BY created_at DESC LIMIT ?`), organizationID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	grants := make([]domain.EmergencyAccessGrant, 0)
	for rows.Next() {
		var grant domain.EmergencyAccessGrant
		var targetUserID, revokedBy, revokeReason sql.NullString
		var revokedAt sql.NullTime
		var keys string
		if err := rows.Scan(&grant.ID, &grant.OrganizationID, &grant.RequesterID, &targetUserID, &grant.Scope, &grant.ApprovalID, &grant.Reason, &keys, &grant.RequestedAt, &grant.ExpiresAt, &revokedAt, &revokedBy, &revokeReason, &grant.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(keys), &grant.PrivilegeKeys); err != nil {
			return nil, fmt.Errorf("decode emergency access privileges: %w", err)
		}
		grant.TargetUserID, grant.RevokedBy, grant.RevokeReason = targetUserID.String, revokedBy.String, revokeReason.String
		if revokedAt.Valid {
			value := revokedAt.Time
			grant.RevokedAt = &value
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func (r *IdentityRepo) RevokeEmergencyAccess(ctx context.Context, organizationID, grantID, actorID, reason string) error {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE emergency_access_grants SET revoked_at=?,revoked_by=?,revoke_reason=? WHERE id=? AND organization_id=? AND revoked_at IS NULL`), time.Now().UTC(), actorID, reason, grantID, organizationID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
