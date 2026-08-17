package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/forge/internal/domain/identity"
	"github.com/sevoniva-labs/forge/internal/platform/database"
)

type IdentityRepo struct{ db *database.DB }

var ErrLastSystemAdmin = errors.New("cannot remove or disable the last active system administrator")

func NewIdentityRepo(db *database.DB) *IdentityRepo { return &IdentityRepo{db: db} }

func (r *IdentityRepo) dbProvider() string { return r.db.Provider }

type userRow struct {
	User         identity.User
	PasswordHash string
}

func (r *IdentityRepo) EnsureOrganization(ctx context.Context, key, name string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM organizations WHERE org_key=?`), key).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO organizations(id,org_key,name,status,description,max_users,max_active_sessions,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`), id, key, name, "ACTIVE", "", 0, 0, now, now)
	if err == nil {
		return id, nil
	}
	// Another instance may have inserted the same bootstrap record between the
	// SELECT and INSERT. Re-read by the natural key so startup remains
	// idempotent across rolling deployments and concurrent Pod starts.
	var existing string
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM organizations WHERE org_key=?`), key).Scan(&existing); readErr == nil {
		return existing, nil
	}
	return "", err
}
func (r *IdentityRepo) EnsureRole(ctx context.Context, orgID, key, name string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, key).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO roles(id,organization_id,role_key,name,created_at) VALUES(?,?,?,?,?)`), id, orgID, key, name, time.Now().UTC())
	if err == nil {
		return id, nil
	}
	var existing string
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, key).Scan(&existing); readErr == nil {
		return existing, nil
	}
	return "", err
}
func (r *IdentityRepo) EnsurePermission(ctx context.Context, key, name string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM permissions WHERE permission_key=?`), key).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO permissions(id,permission_key,name,created_at) VALUES(?,?,?,?)`), id, key, name, time.Now().UTC())
	if err == nil {
		return id, nil
	}
	var existing string
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM permissions WHERE permission_key=?`), key).Scan(&existing); readErr == nil {
		return existing, nil
	}
	return "", err
}
func (r *IdentityRepo) GrantPermissionToRole(ctx context.Context, orgID, roleKey, permissionKey string) error {
	var roleID, permissionID string
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, roleKey).Scan(&roleID); err != nil {
		return err
	}
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM permissions WHERE permission_key=?`), permissionKey).Scan(&permissionID); err != nil {
		return err
	}
	var n int
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM role_permissions WHERE role_id=? AND permission_id=?`), roleID, permissionID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO role_permissions(role_id,permission_id) VALUES(?,?)`), roleID, permissionID)
	if err == nil {
		return nil
	}
	// Concurrent seeders can race on the composite primary key. Treat the
	// operation as successful if the relationship now exists.
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM role_permissions WHERE role_id=? AND permission_id=?`), roleID, permissionID).Scan(&n); readErr == nil && n > 0 {
		return nil
	}
	return err
}

func (r *IdentityRepo) UserByLogin(ctx context.Context, orgID, login string) (userRow, error) {
	var out userRow
	var locked sql.NullTime
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,login_name,display_name,password_hash,status,must_change_password,failed_login_count,locked_until,password_changed_at,created_at,updated_at FROM users WHERE organization_id=? AND login_name=?`), orgID, login).
		Scan(&out.User.ID, &out.User.OrganizationID, &out.User.LoginName, &out.User.DisplayName, &out.PasswordHash, &out.User.Status, &out.User.MustChangePassword, &out.User.FailedLoginCount, &locked, &out.User.PasswordChangedAt, &out.User.CreatedAt, &out.User.UpdatedAt)
	if locked.Valid {
		t := locked.Time
		out.User.LockedUntil = &t
	}
	if err == nil {
		out.User.Roles, _ = r.RolesForUser(ctx, out.User.ID)
		out.User.Permissions, _ = r.PermissionsForUser(ctx, out.User.ID)
	}
	return out, err
}
func (r *IdentityRepo) UserByID(ctx context.Context, id string) (identity.User, error) {
	var out identity.User
	var locked sql.NullTime
	var hash string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,login_name,display_name,password_hash,status,must_change_password,failed_login_count,locked_until,password_changed_at,created_at,updated_at FROM users WHERE id=?`), id).
		Scan(&out.ID, &out.OrganizationID, &out.LoginName, &out.DisplayName, &hash, &out.Status, &out.MustChangePassword, &out.FailedLoginCount, &locked, &out.PasswordChangedAt, &out.CreatedAt, &out.UpdatedAt)
	if locked.Valid {
		t := locked.Time
		out.LockedUntil = &t
	}
	if err == nil {
		out.Roles, _ = r.RolesForUser(ctx, id)
		out.Permissions, _ = r.PermissionsForUser(ctx, id)
	}
	return out, err
}
func (r *IdentityRepo) CreateUser(ctx context.Context, orgID, login, display, passwordHash string, mustChange bool) (identity.User, error) {
	now := time.Now().UTC()
	u := identity.User{ID: uuid.NewString(), OrganizationID: orgID, LoginName: login, DisplayName: display, Status: "ACTIVE", MustChangePassword: mustChange, PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO users(id,organization_id,login_name,display_name,password_hash,status,must_change_password,failed_login_count,password_changed_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`), u.ID, orgID, login, display, passwordHash, u.Status, mustChange, 0, now, now, now)
	return u, err
}
func (r *IdentityRepo) CreateUserWithRoles(ctx context.Context, orgID, login, display, passwordHash string, mustChange bool, roles []string) (identity.User, error) {
	now := time.Now().UTC()
	u := identity.User{ID: uuid.NewString(), OrganizationID: orgID, LoginName: login, DisplayName: display, Status: "ACTIVE", MustChangePassword: mustChange, PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now, Roles: roles}
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO users(id,organization_id,login_name,display_name,password_hash,status,must_change_password,failed_login_count,password_changed_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`), u.ID, orgID, login, display, passwordHash, u.Status, mustChange, 0, now, now, now); err != nil {
			return err
		}
		for _, roleKey := range roles {
			var roleID string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, roleKey).Scan(&roleID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_roles(user_id,role_id) VALUES(?,?)`), u.ID, roleID); err != nil {
				return err
			}
		}
		return nil
	})
	return u, err
}
func (r *IdentityRepo) GrantRole(ctx context.Context, userID, roleKey string) error {
	var roleID string
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT r.id FROM roles r JOIN users u ON u.organization_id=r.organization_id WHERE u.id=? AND r.role_key=?`), userID, roleKey).Scan(&roleID); err != nil {
		return err
	}
	var n int
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_roles WHERE user_id=? AND role_id=?`), userID, roleID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_roles(user_id,role_id) VALUES(?,?)`), userID, roleID)
	if err == nil {
		return nil
	}
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_roles WHERE user_id=? AND role_id=?`), userID, roleID).Scan(&n); readErr == nil && n > 0 {
		return nil
	}
	return err
}
func (r *IdentityRepo) RolesForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT r.role_key FROM roles r JOIN user_roles ur ON ur.role_id=r.id WHERE ur.user_id=? ORDER BY r.role_key`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *IdentityRepo) PermissionsForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT DISTINCT p.permission_key FROM permissions p JOIN role_permissions rp ON rp.permission_id=p.id JOIN user_roles ur ON ur.role_id=rp.role_id WHERE ur.user_id=? ORDER BY p.permission_key`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *IdentityRepo) RecordLoginFailure(ctx context.Context, userID string, max int, lock time.Duration) error {
	if max <= 0 {
		max = 1
	}
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		// Row lock prevents lost increments when multiple API replicas receive
		// simultaneous failed login attempts for the same account. FOR UPDATE is
		// supported by PostgreSQL, MySQL/InnoDB and OceanBase MySQL mode.
		var failures int
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT failed_login_count FROM users WHERE id=? FOR UPDATE`), userID).Scan(&failures); err != nil {
			return err
		}
		failures++
		now := time.Now().UTC()
		var until any
		if failures >= max {
			until = now.Add(lock)
			failures = 0
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET failed_login_count=?,locked_until=?,updated_at=? WHERE id=?`), failures, until, now, userID)
		return err
	})
}
func (r *IdentityRepo) ResetLoginFailure(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE users SET failed_login_count=0,locked_until=NULL,updated_at=? WHERE id=?`), time.Now().UTC(), userID)
	return err
}
func (r *IdentityRepo) CreateSession(ctx context.Context, userID, tokenHash string, expires time.Time, ip, ua string) (string, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO sessions(id,user_id,token_hash,expires_at,created_at,last_seen_at,client_ip,user_agent) VALUES(?,?,?,?,?,?,?,?)`), id, userID, tokenHash, expires, now, now, ip, ua)
	return id, err
}
func (r *IdentityRepo) PrincipalBySessionHash(ctx context.Context, hash string) (identity.Principal, error) {
	var p identity.Principal
	var exp time.Time
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT s.id,u.id,u.organization_id,u.login_name,u.display_name,u.must_change_password,u.password_changed_at,s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND u.status='ACTIVE'`), hash).
		Scan(&p.SessionID, &p.UserID, &p.OrganizationID, &p.LoginName, &p.DisplayName, &p.MustChangePassword, &p.PasswordChangedAt, &exp)
	if err != nil {
		return p, err
	}
	p.Type = "USER"
	if time.Now().After(exp) {
		_ = r.DeleteSessionByID(ctx, p.SessionID)
		return p, sql.ErrNoRows
	}
	p.Roles, _ = r.RolesForUser(ctx, p.UserID)
	p.Permissions, _ = r.PermissionsForUser(ctx, p.UserID)
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE sessions SET last_seen_at=? WHERE id=?`), time.Now().UTC(), p.SessionID)
	return p, nil
}
func (r *IdentityRepo) DeleteSessionByHash(ctx context.Context, hash string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE token_hash=?`), hash)
	return err
}
func (r *IdentityRepo) DeleteSessionByID(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE id=?`), id)
	return err
}
func (r *IdentityRepo) PurgeExpiredSessions(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE expires_at<?`), time.Now().UTC())
	return err
}

func (r *IdentityRepo) PasswordHashByID(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT password_hash FROM users WHERE id=?`), userID).Scan(&hash)
	return hash, err
}
func (r *IdentityRepo) PasswordHistory(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT password_hash FROM password_history WHERE user_id=? ORDER BY changed_at DESC LIMIT ?`), userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func (r *IdentityRepo) UpdatePasswordAndRevokeOtherSessions(ctx context.Context, userID, keepSessionID, oldHash, newHash string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO password_history(id,user_id,password_hash,changed_at) VALUES(?,?,?,?)`), uuid.NewString(), userID, oldHash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET password_hash=?,must_change_password=false,failed_login_count=0,locked_until=NULL,password_changed_at=?,updated_at=? WHERE id=?`), newHash, now, now, userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE user_id=? AND id<>?`), userID, keepSessionID)
		return err
	})
}
func (r *IdentityRepo) ListUsers(ctx context.Context, orgID string, limit int) ([]identity.User, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,login_name,display_name,status,must_change_password,locked_until,password_changed_at,created_at,updated_at FROM users WHERE organization_id=? ORDER BY created_at DESC LIMIT ?`), orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []identity.User
	for rows.Next() {
		var u identity.User
		var locked sql.NullTime
		if err := rows.Scan(&u.ID, &u.OrganizationID, &u.LoginName, &u.DisplayName, &u.Status, &u.MustChangePassword, &locked, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if locked.Valid {
			x := locked.Time
			u.LockedUntil = &x
		}
		u.Roles, _ = r.RolesForUser(ctx, u.ID)
		u.Permissions, _ = r.PermissionsForUser(ctx, u.ID)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) CreateAPIToken(ctx context.Context, userID, name, prefix, tokenHash string, scopes []string, expiresAt *time.Time) (identity.APIToken, error) {
	user, err := r.UserByID(ctx, userID)
	if err != nil {
		return identity.APIToken{}, err
	}
	raw, _ := json.Marshal(scopes)
	now := time.Now().UTC()
	t := identity.APIToken{ID: uuid.NewString(), Name: name, Prefix: prefix, Scopes: scopes, ExpiresAt: expiresAt, CreatedAt: now}
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO api_tokens(id,organization_id,user_id,name,token_prefix,token_hash,scopes_json,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,?)`), t.ID, user.OrganizationID, userID, name, prefix, tokenHash, string(raw), expiresAt, now)
	return t, err
}

func (r *IdentityRepo) ListAPITokens(ctx context.Context, userID string) ([]identity.APIToken, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,name,token_prefix,scopes_json,expires_at,last_used_at,created_at FROM api_tokens WHERE user_id=? AND revoked_at IS NULL ORDER BY created_at DESC LIMIT 200`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []identity.APIToken
	for rows.Next() {
		var t identity.APIToken
		var scopesRaw string
		var expires, last sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.Prefix, &scopesRaw, &expires, &last, &t.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(scopesRaw), &t.Scopes)
		if expires.Valid {
			x := expires.Time
			t.ExpiresAt = &x
		}
		if last.Valid {
			x := last.Time
			t.LastUsedAt = &x
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) RevokeAPIToken(ctx context.Context, userID, tokenID string) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE api_tokens SET revoked_at=? WHERE id=? AND user_id=? AND revoked_at IS NULL`), time.Now().UTC(), tokenID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *IdentityRepo) PrincipalByAPITokenHash(ctx context.Context, hash string) (identity.Principal, error) {
	var p identity.Principal
	var scopesRaw string
	var expires sql.NullTime
	var tokenID string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT t.id,u.id,u.organization_id,u.login_name,u.display_name,u.must_change_password,u.password_changed_at,t.scopes_json,t.expires_at FROM api_tokens t JOIN users u ON u.id=t.user_id WHERE t.token_hash=? AND t.revoked_at IS NULL AND u.status='ACTIVE'`), hash).
		Scan(&tokenID, &p.UserID, &p.OrganizationID, &p.LoginName, &p.DisplayName, &p.MustChangePassword, &p.PasswordChangedAt, &scopesRaw, &expires)
	if err != nil {
		return p, err
	}
	if expires.Valid && time.Now().After(expires.Time) {
		return p, sql.ErrNoRows
	}
	p.Type = "TOKEN"
	_ = json.Unmarshal([]byte(scopesRaw), &p.Scopes)
	p.Roles, _ = r.RolesForUser(ctx, p.UserID)
	p.Permissions, _ = r.PermissionsForUser(ctx, p.UserID)
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE api_tokens SET last_used_at=? WHERE id=?`), time.Now().UTC(), tokenID)
	return p, nil
}

func (r *IdentityRepo) OrganizationByID(ctx context.Context, orgID string) (identity.Organization, error) {
	var out identity.Organization
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,org_key,name,status,description,max_users,max_active_sessions,created_at,updated_at FROM organizations WHERE id=?`), orgID).
		Scan(&out.ID, &out.Key, &out.Name, &out.Status, &out.Description, &out.MaxUsers, &out.MaxSessions, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r *IdentityRepo) UpdateOrganization(ctx context.Context, orgID string, req identity.Organization) (identity.Organization, error) {
	if req.Status != "ACTIVE" && req.Status != "DISABLED" {
		return identity.Organization{}, errors.New("invalid organization status")
	}
	if req.Name = strings.TrimSpace(req.Name); req.Name == "" {
		return identity.Organization{}, errors.New("invalid organization name")
	}
	if req.Description = strings.TrimSpace(req.Description); req.Description == "" {
		req.Description = ""
	}
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE organizations SET name=?,description=?,status=?,max_users=?,max_active_sessions=?,updated_at=? WHERE id=?`), req.Name, req.Description, req.Status, req.MaxUsers, req.MaxSessions, time.Now().UTC(), orgID); err != nil {
		return identity.Organization{}, err
	}
	return r.OrganizationByID(ctx, orgID)
}

func (r *IdentityRepo) ListUserSessionIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id FROM sessions WHERE user_id=? AND expires_at>? ORDER BY created_at ASC`), userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) SecuritySettings(ctx context.Context, orgID string) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT setting_key,setting_value FROM system_settings WHERE organization_id=?`), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

func (r *IdentityRepo) SetSecuritySettings(ctx context.Context, orgID, updatedBy string, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		upsert := `INSERT INTO system_settings(organization_id,setting_key,setting_value,updated_at,updated_by) VALUES(?,?,?,?,?)
			ON CONFLICT (organization_id, setting_key) DO UPDATE
			SET setting_value=EXCLUDED.setting_value,updated_at=EXCLUDED.updated_at,updated_by=EXCLUDED.updated_by`
		if r.dbProvider() != "postgres" {
			upsert = `INSERT INTO system_settings(organization_id,setting_key,setting_value,updated_at,updated_by) VALUES(?,?,?,?,?)
				ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value),updated_at=VALUES(updated_at),updated_by=VALUES(updated_by)`
		}
		q := r.db.Rebind(upsert)
		for key, value := range values {
			if _, err := tx.ExecContext(ctx, q, orgID, key, value, now, updatedBy); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ListPermissions(ctx context.Context) ([]identity.Permission, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,permission_key,name,created_at FROM permissions ORDER BY permission_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []identity.Permission
	for rows.Next() {
		var item identity.Permission
		if err := rows.Scan(&item.ID, &item.Key, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) PermissionsForRole(ctx context.Context, roleID string) ([]identity.Permission, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT p.id,p.permission_key,p.name,p.created_at
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id=p.id
		WHERE rp.role_id=? ORDER BY p.permission_key`), roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []identity.Permission
	for rows.Next() {
		var item identity.Permission
		if err := rows.Scan(&item.ID, &item.Key, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) ListRoles(ctx context.Context, orgID string) ([]identity.Role, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,role_key,name,created_at FROM roles WHERE organization_id=? ORDER BY role_key`), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []identity.Role
	for rows.Next() {
		var item identity.Role
		if err := rows.Scan(&item.ID, &item.Key, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Permissions, _ = r.PermissionsForRole(ctx, item.ID)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) ReplaceRolePermissions(ctx context.Context, orgID, roleKey string, permissionKeys []string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var roleID string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, roleKey).Scan(&roleID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM role_permissions WHERE role_id=?`), roleID); err != nil {
			return err
		}
		for _, key := range permissionKeys {
			var permissionID string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM permissions WHERE permission_key=?`), key).Scan(&permissionID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO role_permissions(role_id,permission_id) VALUES(?,?)`), roleID, permissionID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ReplaceUserRoles(ctx context.Context, orgID, userID string, roleKeys []string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var actualOrg, status string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id,status FROM users WHERE id=? FOR UPDATE`), userID).Scan(&actualOrg, &status); err != nil {
			return err
		}
		if actualOrg != orgID {
			return sql.ErrNoRows
		}

		var currentlySystemAdmin int
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_roles ur JOIN roles ro ON ro.id=ur.role_id WHERE ur.user_id=? AND ro.organization_id=? AND ro.role_key='system_admin'`), userID, orgID).Scan(&currentlySystemAdmin); err != nil {
			return err
		}
		newSystemAdmin := false
		for _, key := range roleKeys {
			if key == "system_admin" {
				newSystemAdmin = true
				break
			}
		}
		if status == "ACTIVE" && currentlySystemAdmin > 0 && !newSystemAdmin {
			n, err := lockActiveSystemAdmins(ctx, r.db, tx, orgID)
			if err != nil {
				return err
			}
			if n <= 1 {
				return ErrLastSystemAdmin
			}
		}

		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM user_roles WHERE user_id=?`), userID); err != nil {
			return err
		}
		for _, key := range roleKeys {
			var roleID string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, key).Scan(&roleID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_roles(user_id,role_id) VALUES(?,?)`), userID, roleID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ListSessions(ctx context.Context, orgID string, limit int) ([]identity.Session, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT s.id,u.id,u.login_name,u.display_name,s.expires_at,s.created_at,s.last_seen_at,s.client_ip,s.user_agent
		FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE u.organization_id=? AND s.expires_at>? ORDER BY s.last_seen_at DESC LIMIT ?`), orgID, time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []identity.Session
	for rows.Next() {
		var item identity.Session
		if err := rows.Scan(&item.ID, &item.UserID, &item.LoginName, &item.DisplayName, &item.ExpiresAt, &item.CreatedAt, &item.LastSeenAt, &item.ClientIP, &item.UserAgent); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) RevokeSession(ctx context.Context, orgID, sessionID string) error {
	var actualOrg string
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT u.organization_id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id=?`), sessionID).Scan(&actualOrg); err != nil {
		return err
	}
	if actualOrg != orgID {
		return sql.ErrNoRows
	}
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE id=?`), sessionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *IdentityRepo) SetUserStatus(ctx context.Context, orgID, userID, status string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var actualOrg, currentStatus string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id,status FROM users WHERE id=? FOR UPDATE`), userID).Scan(&actualOrg, &currentStatus); err != nil {
			return err
		}
		if actualOrg != orgID {
			return sql.ErrNoRows
		}
		if currentStatus == "ACTIVE" && status != "ACTIVE" {
			var systemAdmin int
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_roles ur JOIN roles ro ON ro.id=ur.role_id WHERE ur.user_id=? AND ro.organization_id=? AND ro.role_key='system_admin'`), userID, orgID).Scan(&systemAdmin); err != nil {
				return err
			}
			if systemAdmin > 0 {
				n, err := lockActiveSystemAdmins(ctx, r.db, tx, orgID)
				if err != nil {
					return err
				}
				if n <= 1 {
					return ErrLastSystemAdmin
				}
			}
		}
		res, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET status=?,updated_at=? WHERE id=? AND organization_id=?`), status, time.Now().UTC(), userID, orgID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return sql.ErrNoRows
		}
		if status != "ACTIVE" {
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE user_id=?`), userID); err != nil {
				return err
			}
		}
		return nil
	})
}

func lockActiveSystemAdmins(ctx context.Context, db *database.DB, tx *sql.Tx, orgID string) (int, error) {
	rows, err := tx.QueryContext(ctx, db.Rebind(`SELECT u.id FROM users u JOIN user_roles ur ON ur.user_id=u.id JOIN roles ro ON ro.id=ur.role_id WHERE u.organization_id=? AND u.status='ACTIVE' AND ro.organization_id=? AND ro.role_key='system_admin' FOR UPDATE`), orgID, orgID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		n++
	}
	return n, rows.Err()
}

func (r *IdentityRepo) UnlockUser(ctx context.Context, orgID, userID string) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE users SET failed_login_count=0,locked_until=NULL,updated_at=? WHERE id=? AND organization_id=?`), time.Now().UTC(), userID, orgID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *IdentityRepo) AdminResetPassword(ctx context.Context, orgID, userID, oldHash, newHash string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var actualOrg string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id FROM users WHERE id=?`), userID).Scan(&actualOrg); err != nil {
			return err
		}
		if actualOrg != orgID {
			return sql.ErrNoRows
		}
		now := time.Now().UTC()
		if oldHash != "" {
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO password_history(id,user_id,password_hash,changed_at) VALUES(?,?,?,?)`), uuid.NewString(), userID, oldHash, now); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET password_hash=?,must_change_password=true,failed_login_count=0,locked_until=NULL,password_changed_at=?,updated_at=? WHERE id=?`), newHash, now, now, userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE user_id=?`), userID)
		return err
	})
}
