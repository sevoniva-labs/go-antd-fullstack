package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sevoniva-labs/forge/internal/adapters/repository"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
	"github.com/sevoniva-labs/forge/internal/platform/security/password"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrLocked = errors.New("account locked")
var ErrDisabled = errors.New("account disabled")
var ErrInvalidRole = errors.New("invalid role")
var ErrInvalidLoginName = errors.New("invalid login name")
var ErrPasswordPolicy = errors.New("password policy violation")
var ErrPasswordReused = errors.New("password was used recently")
var ErrLastSystemAdmin = repository.ErrLastSystemAdmin

type Options struct {
	MinLength     int
	RequireUpper  bool
	RequireLower  bool
	RequireDigit  bool
	RequireSymbol bool
	History       int
	MaxAgeDays    int
	SessionTTL    time.Duration
	MaxFailures   int
	LockDuration  time.Duration
}

type Service struct {
	repo         *repository.IdentityRepo
	hasher       password.Hasher
	policy       password.Policy
	sessionTTL   time.Duration
	maxFailures  int
	lockDuration time.Duration
	history      int
	maxAge       time.Duration
}

func NewService(repo *repository.IdentityRepo, opt Options) *Service {
	return &Service{
		repo: repo, hasher: password.DefaultHasher(),
		policy:     password.Policy{MinLength: opt.MinLength, RequireUpper: opt.RequireUpper, RequireLower: opt.RequireLower, RequireDigit: opt.RequireDigit, RequireSymbol: opt.RequireSymbol},
		sessionTTL: opt.SessionTTL, maxFailures: opt.MaxFailures, lockDuration: opt.LockDuration, history: opt.History,
		maxAge: time.Duration(opt.MaxAgeDays) * 24 * time.Hour,
	}
}

var basePermissions = []struct{ Key, Name string }{
	{"system.user.read", "查看用户"}, {"system.user.create", "创建用户"}, {"system.user.update", "修改用户"}, {"system.user.role.manage", "分配用户角色"},
	{"system.role.read", "查看角色权限"}, {"system.role.manage", "管理角色权限"},
	{"system.organization.read", "查看组织信息"},
	{"system.session.read", "查看在线会话"}, {"system.session.revoke", "强制下线会话"},
	{"system.audit.read", "查看审计日志"}, {"system.audit.export", "导出审计日志"},
	{"system.config.read", "查看系统配置"}, {"system.security.manage", "管理安全配置"},
}

func (s *Service) Bootstrap(ctx context.Context, orgKey, orgName, admin, passwordRaw string) error {
	orgID, err := s.repo.EnsureOrganization(ctx, orgKey, orgName)
	if err != nil {
		return err
	}
	for _, r := range []struct{ k, n string }{{"system_admin", "系统管理员"}, {"security_admin", "安全管理员"}, {"auditor", "审计员"}, {"user", "普通用户"}} {
		if _, err = s.repo.EnsureRole(ctx, orgID, r.k, r.n); err != nil {
			return err
		}
	}
	for _, p := range basePermissions {
		if _, err = s.repo.EnsurePermission(ctx, p.Key, p.Name); err != nil {
			return err
		}
	}
	// Keep system_admin as implicit superuser in code; seed explicit grants for
	// other built-in roles to make the model extensible without hard-coding
	// every endpoint to a role name.
	for _, k := range []string{"system.user.read", "system.role.read", "system.organization.read", "system.session.read", "system.session.revoke", "system.audit.read", "system.config.read"} {
		if err = s.repo.GrantPermissionToRole(ctx, orgID, "security_admin", k); err != nil {
			return err
		}
	}
	for _, k := range []string{"system.audit.read", "system.audit.export"} {
		if err = s.repo.GrantPermissionToRole(ctx, orgID, "auditor", k); err != nil {
			return err
		}
	}
	if admin == "" {
		return nil
	}
	if _, err = s.repo.UserByLogin(ctx, orgID, admin); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err = s.policy.Validate(passwordRaw); err != nil {
		return fmt.Errorf("bootstrap password: %w", err)
	}
	h, err := s.hasher.Hash(passwordRaw)
	if err != nil {
		return err
	}
	u, err := s.repo.CreateUser(ctx, orgID, admin, "Administrator", h, true)
	if err != nil {
		// Multiple replicas can bootstrap concurrently. If another replica won
		// the unique (organization_id, login_name) race, re-read the account and
		// finish the idempotent role grant instead of failing this replica.
		existing, readErr := s.repo.UserByLogin(ctx, orgID, admin)
		if readErr != nil {
			return err
		}
		return s.repo.GrantRole(ctx, existing.User.ID, "system_admin")
	}
	return s.repo.GrantRole(ctx, u.ID, "system_admin")
}
func (s *Service) Login(ctx context.Context, orgID, login, raw, ip, ua string) (domain.Principal, string, string, time.Time, error) {
	row, err := s.repo.UserByLogin(ctx, orgID, strings.TrimSpace(login))
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	if row.User.Status != "ACTIVE" {
		return domain.Principal{}, "", "", time.Time{}, ErrDisabled
	}
	if row.User.LockedUntil != nil && time.Now().Before(*row.User.LockedUntil) {
		return domain.Principal{}, "", "", *row.User.LockedUntil, ErrLocked
	}
	if !s.hasher.Verify(raw, row.PasswordHash) {
		_ = s.repo.RecordLoginFailure(ctx, row.User.ID, s.maxFailures, s.lockDuration)
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	_ = s.repo.ResetLoginFailure(ctx, row.User.ID)
	token, err := randomToken(32)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(s.sessionTTL)
	sessionID, err := s.repo.CreateSession(ctx, row.User.ID, hashToken(token), expires, ip, ua)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	mustChange := row.User.MustChangePassword || s.passwordExpired(row.User.PasswordChangedAt)
	p := domain.Principal{Type: "USER", UserID: row.User.ID, OrganizationID: row.User.OrganizationID, LoginName: row.User.LoginName, DisplayName: row.User.DisplayName, Roles: row.User.Roles, Permissions: row.User.Permissions, MustChangePassword: mustChange, SessionID: sessionID, PasswordChangedAt: row.User.PasswordChangedAt}
	return p, token, csrf, expires, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (domain.Principal, error) {
	if token == "" {
		return domain.Principal{}, sql.ErrNoRows
	}
	p, err := s.repo.PrincipalBySessionHash(ctx, hashToken(token))
	if err == nil && s.passwordExpired(p.PasswordChangedAt) {
		p.MustChangePassword = true
	}
	return p, err
}
func (s *Service) passwordExpired(t time.Time) bool {
	return s.maxAge > 0 && !t.IsZero() && time.Since(t) > s.maxAge
}
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSessionByHash(ctx, hashToken(token))
}
func (s *Service) ListUsers(ctx context.Context, orgID string) ([]domain.User, error) {
	return s.repo.ListUsers(ctx, orgID, 200)
}
func (s *Service) CreateUser(ctx context.Context, orgID, login, display, raw string, roles []string) (domain.User, error) {
	login = strings.TrimSpace(login)
	display = strings.TrimSpace(display)
	if login == "" || len(login) > 120 {
		return domain.User{}, ErrInvalidLoginName
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}
	allowed := map[string]struct{}{"system_admin": {}, "security_admin": {}, "auditor": {}, "user": {}}
	for _, r := range roles {
		if _, ok := allowed[r]; !ok {
			return domain.User{}, ErrInvalidRole
		}
	}
	if err := s.policy.Validate(raw); err != nil {
		return domain.User{}, fmt.Errorf("%w: %v", ErrPasswordPolicy, err)
	}
	h, err := s.hasher.Hash(raw)
	if err != nil {
		return domain.User{}, err
	}
	return s.repo.CreateUserWithRoles(ctx, orgID, login, display, h, true, roles)
}
func (s *Service) ChangePassword(ctx context.Context, userID, sessionID, current, next string) error {
	if err := s.policy.Validate(next); err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordPolicy, err)
	}
	currentHash, err := s.repo.PasswordHashByID(ctx, userID)
	if err != nil {
		return err
	}
	if !s.hasher.Verify(current, currentHash) {
		return ErrInvalidCredentials
	}
	if s.hasher.Verify(next, currentHash) {
		return ErrPasswordReused
	}
	if s.history > 0 {
		history, err := s.repo.PasswordHistory(ctx, userID, s.history)
		if err != nil {
			return err
		}
		for _, old := range history {
			if s.hasher.Verify(next, old) {
				return ErrPasswordReused
			}
		}
	}
	nextHash, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordAndRevokeOtherSessions(ctx, userID, sessionID, currentHash, nextHash)
}
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func hashToken(v string) string { x := sha256.Sum256([]byte(v)); return hex.EncodeToString(x[:]) }

func (s *Service) CreateAPIToken(ctx context.Context, userID, name string, scopes []string, ttl time.Duration) (domain.APIToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return domain.APIToken{}, "", errors.New("invalid token name")
	}
	u, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	if u.MustChangePassword || s.passwordExpired(u.PasswordChangedAt) {
		return domain.APIToken{}, "", ErrPasswordPolicy
	}
	allowed := map[string]struct{}{}
	for _, p := range u.Permissions {
		allowed[p] = struct{}{}
	}
	if u.HasRole("system_admin") {
		allowed["*"] = struct{}{}
	}
	if len(scopes) == 0 {
		if u.HasRole("system_admin") {
			scopes = []string{"*"}
		} else {
			scopes = append([]string(nil), u.Permissions...)
		}
	}
	for _, scope := range scopes {
		if _, ok := allowed[scope]; !ok {
			return domain.APIToken{}, "", ErrInvalidRole
		}
	}
	if ttl <= 0 {
		ttl = 90 * 24 * time.Hour
	}
	if ttl > 365*24*time.Hour {
		return domain.APIToken{}, "", errors.New("api token ttl exceeds 365 days")
	}
	expires := time.Now().UTC().Add(ttl)
	random, err := randomToken(32)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	raw := "fg_" + random
	prefix := raw
	if len(prefix) > 15 {
		prefix = prefix[:15]
	}
	t, err := s.repo.CreateAPIToken(ctx, userID, name, prefix, hashToken(raw), scopes, &expires)
	return t, raw, err
}
func (s *Service) ListAPITokens(ctx context.Context, userID string) ([]domain.APIToken, error) {
	return s.repo.ListAPITokens(ctx, userID)
}
func (s *Service) RevokeAPIToken(ctx context.Context, userID, tokenID string) error {
	return s.repo.RevokeAPIToken(ctx, userID, tokenID)
}
func (s *Service) AuthenticateAPIToken(ctx context.Context, raw string) (domain.Principal, error) {
	if !strings.HasPrefix(raw, "fg_") {
		return domain.Principal{}, ErrInvalidCredentials
	}
	p, err := s.repo.PrincipalByAPITokenHash(ctx, hashToken(raw))
	if err != nil {
		return domain.Principal{}, ErrInvalidCredentials
	}
	if p.MustChangePassword || s.passwordExpired(p.PasswordChangedAt) {
		return domain.Principal{}, ErrInvalidCredentials
	}
	return p, nil
}

func (s *Service) Organization(ctx context.Context, orgID string) (domain.Organization, error) {
	return s.repo.OrganizationByID(ctx, orgID)
}

func (s *Service) ListRoles(ctx context.Context, orgID string) ([]domain.Role, error) {
	return s.repo.ListRoles(ctx, orgID)
}

func (s *Service) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *Service) UpdateRolePermissions(ctx context.Context, orgID, roleKey string, permissionKeys []string) error {
	roleKey = strings.TrimSpace(roleKey)
	if roleKey == "" || roleKey == "system_admin" {
		return ErrInvalidRole
	}
	allowed := map[string]struct{}{}
	items, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		allowed[item.Key] = struct{}{}
	}
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(permissionKeys))
	for _, key := range permissionKeys {
		key = strings.TrimSpace(key)
		if _, ok := allowed[key]; !ok {
			return ErrInvalidRole
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, key)
	}
	return s.repo.ReplaceRolePermissions(ctx, orgID, roleKey, clean)
}

func (s *Service) UpdateUserRoles(ctx context.Context, orgID, userID string, roleKeys []string) error {
	if len(roleKeys) == 0 {
		roleKeys = []string{"user"}
	}
	roles, err := s.repo.ListRoles(ctx, orgID)
	if err != nil {
		return err
	}
	allowed := map[string]struct{}{}
	for _, role := range roles {
		allowed[role.Key] = struct{}{}
	}
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(roleKeys))
	for _, key := range roleKeys {
		key = strings.TrimSpace(key)
		if _, ok := allowed[key]; !ok {
			return ErrInvalidRole
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, key)
	}
	return s.repo.ReplaceUserRoles(ctx, orgID, userID, clean)
}

func (s *Service) ListSessions(ctx context.Context, orgID, currentSessionID string) ([]domain.Session, error) {
	items, err := s.repo.ListSessions(ctx, orgID, 500)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Current = items[i].ID == currentSessionID
	}
	return items, nil
}

func (s *Service) RevokeSession(ctx context.Context, orgID, sessionID, currentSessionID string) error {
	if strings.TrimSpace(sessionID) == "" || sessionID == currentSessionID {
		return errors.New("cannot revoke current session")
	}
	return s.repo.RevokeSession(ctx, orgID, sessionID)
}

func (s *Service) SetUserStatus(ctx context.Context, orgID, userID, status string) error {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "ACTIVE" && status != "DISABLED" {
		return errors.New("invalid user status")
	}
	return s.repo.SetUserStatus(ctx, orgID, userID, status)
}

func (s *Service) UnlockUser(ctx context.Context, orgID, userID string) error {
	return s.repo.UnlockUser(ctx, orgID, userID)
}

func (s *Service) AdminResetPassword(ctx context.Context, orgID, userID, next string) error {
	if err := s.policy.Validate(next); err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordPolicy, err)
	}
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.OrganizationID != orgID {
		return sql.ErrNoRows
	}
	currentHash, err := s.repo.PasswordHashByID(ctx, userID)
	if err != nil {
		return err
	}
	if s.hasher.Verify(next, currentHash) {
		return ErrPasswordReused
	}
	nextHash, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	return s.repo.AdminResetPassword(ctx, orgID, userID, currentHash, nextHash)
}
