package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
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
var ErrGrantCeiling = errors.New("grant ceiling exceeded")
var ErrInvalidLoginName = errors.New("invalid login name")
var ErrPasswordPolicy = errors.New("password policy violation")
var ErrPasswordReused = errors.New("password was used recently")
var ErrInvalidSecurityPolicy = errors.New("invalid security policy")
var ErrLastSystemAdmin = repository.ErrLastSystemAdmin
var ErrPasswordStateChanged = repository.ErrPasswordStateChanged
var ErrInteractiveSessionRequired = errors.New("interactive user session required")
var ErrInvalidDepartment = errors.New("invalid department")

type resolvedPolicy struct {
	passwordPolicy password.Policy
	sessionTTL     time.Duration
	maxFailures    int
	lockDuration   time.Duration
	maxAge         time.Duration
	history        int
	maxConcurrent  int
}

const (
	minimumPasswordLength     = 12
	minimumPasswordHistory    = 5
	maximumPasswordAgeDays    = 90
	maximumLoginFailures      = 5
	minimumLoginLockDuration  = 15 * time.Minute
	maximumSessionTTL         = 12 * time.Hour
	maximumConcurrentSessions = 5
)

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
	{"system.organization.read", "查看组织信息"}, {"system.organization.manage", "管理组织信息"},
	{"system.department.read", "查看部门"}, {"system.department.manage", "管理部门"},
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
	for _, k := range []string{"system.user.read", "system.role.read", "system.organization.read", "system.organization.manage", "system.department.read", "system.department.manage", "system.session.read", "system.session.revoke", "system.config.read", "system.security.manage"} {
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
		return s.finalizeBootstrapDefaults(ctx, orgID)
	}
	if _, err = s.repo.UserByLogin(ctx, orgID, admin); err == nil {
		return s.finalizeBootstrapDefaults(ctx, orgID)
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
	if err = s.repo.GrantRole(ctx, u.ID, "system_admin"); err != nil {
		return err
	}
	return s.finalizeBootstrapDefaults(ctx, orgID)
}

func (s *Service) enforceOrganizationActive(ctx context.Context, orgID string) error {
	org, err := s.repo.OrganizationByID(ctx, orgID)
	if err != nil {
		return err
	}
	if strings.ToUpper(org.Status) != "ACTIVE" {
		return ErrDisabled
	}
	return nil
}

func (s *Service) defaultPolicy() resolvedPolicy {
	return resolvedPolicy{
		passwordPolicy: s.policy,
		sessionTTL:     s.sessionTTL,
		maxFailures:    s.maxFailures,
		lockDuration:   s.lockDuration,
		maxAge:         s.maxAge,
		history:        s.history,
		maxConcurrent:  maximumConcurrentSessions,
	}
}

func (s *Service) resolveSecurityPolicy(ctx context.Context, orgID string) (resolvedPolicy, error) {
	out := s.defaultPolicy()
	if orgID == "" {
		return out, validateResolvedPolicy(out)
	}
	settings, err := s.repo.SecuritySettings(ctx, orgID)
	if err != nil {
		return resolvedPolicy{}, err
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordMinLength]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 1 {
			return resolvedPolicy{}, fmt.Errorf("%w: password_min_length", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.MinLength = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordRequireUpper]); v != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			return resolvedPolicy{}, fmt.Errorf("%w: password_require_upper", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.RequireUpper = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordRequireLower]); v != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			return resolvedPolicy{}, fmt.Errorf("%w: password_require_lower", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.RequireLower = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordRequireDigit]); v != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			return resolvedPolicy{}, fmt.Errorf("%w: password_require_digit", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.RequireDigit = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordRequireSymbol]); v != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			return resolvedPolicy{}, fmt.Errorf("%w: password_require_symbol", ErrInvalidSecurityPolicy)
		}
		out.passwordPolicy.RequireSymbol = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordHistory]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: password_history", ErrInvalidSecurityPolicy)
		}
		out.history = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingPasswordMaxAgeDays]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: password_max_age_days", ErrInvalidSecurityPolicy)
		}
		out.maxAge = time.Duration(parsed) * 24 * time.Hour
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingLoginMaxFailures]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: login_max_failures", ErrInvalidSecurityPolicy)
		}
		out.maxFailures = parsed
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingLoginLockDurationSec]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: login_lock_duration_seconds", ErrInvalidSecurityPolicy)
		}
		out.lockDuration = time.Duration(parsed) * time.Second
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingSessionTTLSeconds]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed <= 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: session_ttl_seconds", ErrInvalidSecurityPolicy)
		}
		out.sessionTTL = time.Duration(parsed) * time.Second
	}
	if v := strings.TrimSpace(settings[domain.SecuritySettingMaxConcurrentSessions]); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 0 {
			return resolvedPolicy{}, fmt.Errorf("%w: max_active_sessions", ErrInvalidSecurityPolicy)
		}
		out.maxConcurrent = parsed
	}
	if err := validateResolvedPolicy(out); err != nil {
		return resolvedPolicy{}, err
	}
	return out, nil
}

func (s *Service) finalizeBootstrapDefaults(ctx context.Context, orgID string) error {
	policy := s.defaultPolicy()
	existing, err := s.repo.SecuritySettings(ctx, orgID)
	if err != nil {
		return err
	}
	changes := map[string]string{
		domain.SecuritySettingPasswordMinLength:     strconv.Itoa(policy.passwordPolicy.MinLength),
		domain.SecuritySettingPasswordRequireUpper:  strconv.FormatBool(policy.passwordPolicy.RequireUpper),
		domain.SecuritySettingPasswordRequireLower:  strconv.FormatBool(policy.passwordPolicy.RequireLower),
		domain.SecuritySettingPasswordRequireDigit:  strconv.FormatBool(policy.passwordPolicy.RequireDigit),
		domain.SecuritySettingPasswordRequireSymbol: strconv.FormatBool(policy.passwordPolicy.RequireSymbol),
		domain.SecuritySettingPasswordHistory:       strconv.Itoa(policy.history),
		domain.SecuritySettingPasswordMaxAgeDays:    strconv.Itoa(int(policy.maxAge.Hours()) / 24),
		domain.SecuritySettingLoginMaxFailures:      strconv.Itoa(policy.maxFailures),
		domain.SecuritySettingLoginLockDurationSec:  strconv.FormatInt(int64(policy.lockDuration.Seconds()), 10),
		domain.SecuritySettingSessionTTLSeconds:     strconv.FormatInt(int64(policy.sessionTTL.Seconds()), 10),
		domain.SecuritySettingMaxConcurrentSessions: strconv.Itoa(policy.maxConcurrent),
	}
	for key := range changes {
		if _, ok := existing[key]; ok {
			delete(changes, key)
		}
	}
	if len(changes) == 0 {
		return nil
	}
	return s.repo.SetSecuritySettings(ctx, orgID, "system", changes)
}

func (s *Service) SecurityPolicy(ctx context.Context, orgID string) (domain.SecurityPolicy, error) {
	policy, err := s.resolveSecurityPolicy(ctx, orgID)
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	return domain.SecurityPolicy{
		PasswordMinLength:        policy.passwordPolicy.MinLength,
		PasswordRequireUpper:     policy.passwordPolicy.RequireUpper,
		PasswordRequireLower:     policy.passwordPolicy.RequireLower,
		PasswordRequireDigit:     policy.passwordPolicy.RequireDigit,
		PasswordRequireSymbol:    policy.passwordPolicy.RequireSymbol,
		PasswordHistory:          policy.history,
		PasswordMaxAgeDays:       int(policy.maxAge.Hours()) / 24,
		LoginMaxFailures:         policy.maxFailures,
		LoginLockDurationSeconds: int64(policy.lockDuration.Seconds()),
		SessionTTLSeconds:        int64(policy.sessionTTL.Seconds()),
		MaxConcurrentSessions:    policy.maxConcurrent,
	}, nil
}

func (s *Service) UpdateSecurityPolicy(ctx context.Context, orgID, updatedBy string, policy domain.SecurityPolicy) (domain.SecurityPolicy, error) {
	if err := validateSecurityPolicy(policy); err != nil {
		return domain.SecurityPolicy{}, err
	}
	payload := map[string]string{
		domain.SecuritySettingPasswordMinLength:     strconv.Itoa(policy.PasswordMinLength),
		domain.SecuritySettingPasswordRequireUpper:  strconv.FormatBool(policy.PasswordRequireUpper),
		domain.SecuritySettingPasswordRequireLower:  strconv.FormatBool(policy.PasswordRequireLower),
		domain.SecuritySettingPasswordRequireDigit:  strconv.FormatBool(policy.PasswordRequireDigit),
		domain.SecuritySettingPasswordRequireSymbol: strconv.FormatBool(policy.PasswordRequireSymbol),
		domain.SecuritySettingPasswordHistory:       strconv.Itoa(policy.PasswordHistory),
		domain.SecuritySettingPasswordMaxAgeDays:    strconv.Itoa(policy.PasswordMaxAgeDays),
		domain.SecuritySettingLoginMaxFailures:      strconv.Itoa(policy.LoginMaxFailures),
		domain.SecuritySettingLoginLockDurationSec:  strconv.FormatInt(policy.LoginLockDurationSeconds, 10),
		domain.SecuritySettingSessionTTLSeconds:     strconv.FormatInt(policy.SessionTTLSeconds, 10),
		domain.SecuritySettingMaxConcurrentSessions: strconv.Itoa(policy.MaxConcurrentSessions),
	}
	if err := s.repo.SetSecuritySettings(ctx, orgID, updatedBy, payload); err != nil {
		return domain.SecurityPolicy{}, err
	}
	return s.SecurityPolicy(ctx, orgID)
}

func validateResolvedPolicy(policy resolvedPolicy) error {
	return validateSecurityPolicy(domain.SecurityPolicy{
		PasswordMinLength:        policy.passwordPolicy.MinLength,
		PasswordRequireUpper:     policy.passwordPolicy.RequireUpper,
		PasswordRequireLower:     policy.passwordPolicy.RequireLower,
		PasswordRequireDigit:     policy.passwordPolicy.RequireDigit,
		PasswordRequireSymbol:    policy.passwordPolicy.RequireSymbol,
		PasswordHistory:          policy.history,
		PasswordMaxAgeDays:       int(policy.maxAge.Hours()) / 24,
		LoginMaxFailures:         policy.maxFailures,
		LoginLockDurationSeconds: int64(policy.lockDuration.Seconds()),
		SessionTTLSeconds:        int64(policy.sessionTTL.Seconds()),
		MaxConcurrentSessions:    policy.maxConcurrent,
	})
}

func validateSecurityPolicy(policy domain.SecurityPolicy) error {
	switch {
	case policy.PasswordMinLength < minimumPasswordLength:
		return fmt.Errorf("%w: password_min_length must be at least %d", ErrInvalidSecurityPolicy, minimumPasswordLength)
	case !policy.PasswordRequireUpper || !policy.PasswordRequireLower || !policy.PasswordRequireDigit || !policy.PasswordRequireSymbol:
		return fmt.Errorf("%w: password character-class requirements cannot be disabled", ErrInvalidSecurityPolicy)
	case policy.PasswordHistory < minimumPasswordHistory:
		return fmt.Errorf("%w: password_history must be at least %d", ErrInvalidSecurityPolicy, minimumPasswordHistory)
	case policy.PasswordMaxAgeDays < 1 || policy.PasswordMaxAgeDays > maximumPasswordAgeDays:
		return fmt.Errorf("%w: password_max_age_days must be between 1 and %d", ErrInvalidSecurityPolicy, maximumPasswordAgeDays)
	case policy.LoginMaxFailures < 1 || policy.LoginMaxFailures > maximumLoginFailures:
		return fmt.Errorf("%w: login_max_failures must be between 1 and %d", ErrInvalidSecurityPolicy, maximumLoginFailures)
	case time.Duration(policy.LoginLockDurationSeconds)*time.Second < minimumLoginLockDuration:
		return fmt.Errorf("%w: login_lock_duration_seconds must be at least %d", ErrInvalidSecurityPolicy, int64(minimumLoginLockDuration.Seconds()))
	case policy.SessionTTLSeconds < 1 || time.Duration(policy.SessionTTLSeconds)*time.Second > maximumSessionTTL:
		return fmt.Errorf("%w: session_ttl_seconds must be between 1 and %d", ErrInvalidSecurityPolicy, int64(maximumSessionTTL.Seconds()))
	case policy.MaxConcurrentSessions < 1 || policy.MaxConcurrentSessions > maximumConcurrentSessions:
		return fmt.Errorf("%w: max_active_sessions must be between 1 and %d", ErrInvalidSecurityPolicy, maximumConcurrentSessions)
	default:
		return nil
	}
}

func (s *Service) UpdateOrganization(ctx context.Context, orgID string, req domain.Organization) (domain.Organization, error) {
	if req.Name = strings.TrimSpace(req.Name); req.Name == "" {
		return domain.Organization{}, fmt.Errorf("invalid organization name")
	}
	if req.Status == "" {
		existing, err := s.repo.OrganizationByID(ctx, orgID)
		if err != nil {
			return domain.Organization{}, err
		}
		req.Status = existing.Status
	}
	req.Status = strings.ToUpper(req.Status)
	if req.Status != "ACTIVE" && req.Status != "DISABLED" {
		return domain.Organization{}, fmt.Errorf("invalid organization status")
	}
	if req.MaxUsers < 0 || req.MaxSessions < 0 {
		return domain.Organization{}, fmt.Errorf("invalid organization value")
	}
	return s.repo.UpdateOrganization(ctx, orgID, req)
}

func (s *Service) enforceMaxConcurrentSessions(ctx context.Context, userID string, maxSessions int) error {
	if maxSessions <= 0 {
		return nil
	}
	ids, err := s.repo.ListUserSessionIDs(ctx, userID)
	if err != nil {
		return err
	}
	excess := len(ids) - maxSessions
	if excess <= 0 {
		return nil
	}
	for i := 0; i < excess; i++ {
		if err := s.repo.DeleteSessionByID(ctx, ids[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) passwordExpiredAt(t time.Time, maxAge time.Duration) bool {
	return maxAge > 0 && !t.IsZero() && time.Since(t) > maxAge
}

func (s *Service) Login(ctx context.Context, orgID, login, raw, ip, ua string) (domain.Principal, string, string, time.Time, error) {
	row, err := s.repo.UserByLogin(ctx, orgID, strings.TrimSpace(login))
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	org, err := s.repo.OrganizationByID(ctx, row.User.OrganizationID)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if strings.ToUpper(org.Status) != "ACTIVE" {
		return domain.Principal{}, "", "", time.Time{}, ErrDisabled
	}
	policy, err := s.resolveSecurityPolicy(ctx, row.User.OrganizationID)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if row.User.Status != "ACTIVE" {
		return domain.Principal{}, "", "", time.Time{}, ErrDisabled
	}
	if row.User.LockedUntil != nil && time.Now().Before(*row.User.LockedUntil) {
		return domain.Principal{}, "", "", *row.User.LockedUntil, ErrLocked
	}
	if !s.hasher.Verify(raw, row.PasswordHash) {
		if err := s.repo.RecordLoginFailure(ctx, row.User.ID, policy.maxFailures, policy.lockDuration); err != nil {
			return domain.Principal{}, "", "", time.Time{}, err
		}
		return domain.Principal{}, "", "", time.Time{}, ErrInvalidCredentials
	}
	if err := s.repo.ResetLoginFailure(ctx, row.User.ID); err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	token, err := randomToken(32)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(policy.sessionTTL)
	sessionID, err := s.repo.CreateSession(ctx, row.User.ID, hashToken(token), expires, ip, ua)
	if err != nil {
		return domain.Principal{}, "", "", time.Time{}, err
	}
	if err := s.enforceMaxConcurrentSessions(ctx, row.User.ID, policy.maxConcurrent); err != nil {
		_ = s.repo.DeleteSessionByID(ctx, sessionID)
		return domain.Principal{}, "", "", time.Time{}, err
	}
	mustChange := row.User.MustChangePassword || s.passwordExpiredAt(row.User.PasswordChangedAt, policy.maxAge)
	p := domain.Principal{Type: "USER", UserID: row.User.ID, OrganizationID: row.User.OrganizationID, LoginName: row.User.LoginName, DisplayName: row.User.DisplayName, Roles: row.User.Roles, Permissions: row.User.Permissions, MustChangePassword: mustChange, SessionID: sessionID, PasswordChangedAt: row.User.PasswordChangedAt}
	return p, token, csrf, expires, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (domain.Principal, error) {
	if token == "" {
		return domain.Principal{}, sql.ErrNoRows
	}
	p, err := s.repo.PrincipalBySessionHash(ctx, hashToken(token))
	if err == nil {
		org, e := s.repo.OrganizationByID(ctx, p.OrganizationID)
		if e != nil {
			return domain.Principal{}, e
		}
		if strings.ToUpper(org.Status) != "ACTIVE" {
			return domain.Principal{}, ErrDisabled
		}
		policy, policyErr := s.resolveSecurityPolicy(ctx, p.OrganizationID)
		if policyErr != nil {
			return domain.Principal{}, policyErr
		}
		if s.passwordExpiredAt(p.PasswordChangedAt, policy.maxAge) {
			p.MustChangePassword = true
		}
	}
	return p, err
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
func (s *Service) CreateUser(ctx context.Context, actor domain.Principal, orgID, login, display, raw string, roles []string) (domain.User, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.User{}, err
	}
	if _, err := s.repo.OrganizationByID(ctx, orgID); err != nil {
		return domain.User{}, err
	}
	if err := s.enforceOrganizationActive(ctx, orgID); err != nil {
		return domain.User{}, err
	}
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
	if err := enforceRoleMutation(actor, nil, roles); err != nil {
		return domain.User{}, err
	}
	policy, err := s.resolveSecurityPolicy(ctx, orgID)
	if err != nil {
		return domain.User{}, err
	}
	if err := policy.passwordPolicy.Validate(raw); err != nil {
		return domain.User{}, fmt.Errorf("%w: %v", ErrPasswordPolicy, err)
	}
	h, err := s.hasher.Hash(raw)
	if err != nil {
		return domain.User{}, err
	}
	return s.repo.CreateUserWithRoles(ctx, orgID, login, display, h, true, roles)
}
func (s *Service) ChangePassword(ctx context.Context, actor domain.Principal, current, next string) error {
	if err := requireInteractivePrincipal(actor); err != nil {
		return err
	}
	user, err := s.repo.UserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	policy, err := s.resolveSecurityPolicy(ctx, user.OrganizationID)
	if err != nil {
		return err
	}
	if err := s.enforceOrganizationActive(ctx, user.OrganizationID); err != nil {
		return err
	}
	if err := policy.passwordPolicy.Validate(next); err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordPolicy, err)
	}
	currentHash, err := s.repo.PasswordHashByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !s.hasher.Verify(current, currentHash) {
		return ErrInvalidCredentials
	}
	if s.hasher.Verify(next, currentHash) {
		return ErrPasswordReused
	}
	if policy.history > 0 {
		history, err := s.repo.PasswordHistory(ctx, actor.UserID, policy.history)
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
	return s.repo.UpdatePasswordAndRevokeOtherSessions(ctx, actor.UserID, actor.SessionID, currentHash, nextHash)
}
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func hashToken(v string) string { x := sha256.Sum256([]byte(v)); return hex.EncodeToString(x[:]) }

func (s *Service) CreateAPIToken(ctx context.Context, actor domain.Principal, name string, scopes []string, ttl time.Duration) (domain.APIToken, string, error) {
	if err := requireInteractivePrincipal(actor); err != nil {
		return domain.APIToken{}, "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return domain.APIToken{}, "", errors.New("invalid token name")
	}
	u, err := s.repo.UserByID(ctx, actor.UserID)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	policy, err := s.resolveSecurityPolicy(ctx, u.OrganizationID)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	if u.MustChangePassword || s.passwordExpiredAt(u.PasswordChangedAt, policy.maxAge) {
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
	t, err := s.repo.CreateAPIToken(ctx, actor.UserID, name, prefix, hashToken(raw), scopes, &expires)
	return t, raw, err
}
func (s *Service) ListAPITokens(ctx context.Context, actor domain.Principal) ([]domain.APIToken, error) {
	if err := requireInteractivePrincipal(actor); err != nil {
		return nil, err
	}
	return s.repo.ListAPITokens(ctx, actor.UserID)
}
func (s *Service) RevokeAPIToken(ctx context.Context, actor domain.Principal, tokenID string) error {
	if err := requireInteractivePrincipal(actor); err != nil {
		return err
	}
	return s.repo.RevokeAPIToken(ctx, actor.UserID, tokenID)
}

func requireInteractivePrincipal(actor domain.Principal) error {
	if actor.Type != "USER" || actor.UserID == "" || actor.OrganizationID == "" || actor.SessionID == "" {
		return ErrInteractiveSessionRequired
	}
	return nil
}
func (s *Service) AuthenticateAPIToken(ctx context.Context, raw string) (domain.Principal, error) {
	if !strings.HasPrefix(raw, "fg_") {
		return domain.Principal{}, ErrInvalidCredentials
	}
	p, err := s.repo.PrincipalByAPITokenHash(ctx, hashToken(raw))
	if err != nil {
		return domain.Principal{}, ErrInvalidCredentials
	}
	org, err := s.repo.OrganizationByID(ctx, p.OrganizationID)
	if err != nil {
		return domain.Principal{}, ErrInvalidCredentials
	}
	if strings.ToUpper(org.Status) != "ACTIVE" {
		return domain.Principal{}, ErrInvalidCredentials
	}
	policy, err := s.resolveSecurityPolicy(ctx, p.OrganizationID)
	if err != nil {
		return domain.Principal{}, ErrInvalidCredentials
	}
	if p.MustChangePassword || s.passwordExpiredAt(p.PasswordChangedAt, policy.maxAge) {
		return domain.Principal{}, ErrInvalidCredentials
	}
	return p, nil
}

func (s *Service) Organization(ctx context.Context, orgID string) (domain.Organization, error) {
	return s.repo.OrganizationByID(ctx, orgID)
}

func (s *Service) ListDepartments(ctx context.Context, orgID string) ([]domain.Department, error) {
	return s.repo.ListDepartments(ctx, orgID)
}

func (s *Service) CreateDepartment(ctx context.Context, actor domain.Principal, orgID string, req domain.Department) (domain.Department, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.Department{}, err
	}
	req.OrganizationID = orgID
	clean, err := normalizeDepartment(req, true)
	if err != nil {
		return domain.Department{}, err
	}
	items, err := s.repo.ListDepartmentsForUpdate(ctx, orgID)
	if err != nil {
		return domain.Department{}, err
	}
	if err := validateDepartmentHierarchy(items, clean, true); err != nil {
		return domain.Department{}, err
	}
	return s.repo.CreateDepartment(ctx, clean)
}

func (s *Service) UpdateDepartment(ctx context.Context, actor domain.Principal, orgID, departmentID string, req domain.Department) (domain.Department, error) {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return domain.Department{}, err
	}
	departmentID = strings.TrimSpace(departmentID)
	if departmentID == "" {
		return domain.Department{}, ErrInvalidDepartment
	}
	items, err := s.repo.ListDepartmentsForUpdate(ctx, orgID)
	if err != nil {
		return domain.Department{}, err
	}
	var current domain.Department
	found := false
	for _, item := range items {
		if item.ID == departmentID {
			current = item
			found = true
			break
		}
	}
	if !found {
		return domain.Department{}, sql.ErrNoRows
	}
	req.ID = current.ID
	req.OrganizationID = orgID
	req.Key = current.Key
	req.CreatedAt = current.CreatedAt
	clean, err := normalizeDepartment(req, false)
	if err != nil {
		return domain.Department{}, err
	}
	if err := validateDepartmentHierarchy(items, clean, false); err != nil {
		return domain.Department{}, err
	}
	return s.repo.UpdateDepartment(ctx, clean)
}

func normalizeDepartment(req domain.Department, creating bool) (domain.Department, error) {
	req.ID = strings.TrimSpace(req.ID)
	req.OrganizationID = strings.TrimSpace(req.OrganizationID)
	req.ParentID = strings.TrimSpace(req.ParentID)
	req.Key = strings.TrimSpace(req.Key)
	req.Name = strings.TrimSpace(req.Name)
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))
	if creating && req.Status == "" {
		req.Status = "ACTIVE"
	}
	if req.OrganizationID == "" || req.Name == "" || len(req.Name) > 200 || req.SortOrder < 0 || req.SortOrder > 1_000_000 {
		return domain.Department{}, ErrInvalidDepartment
	}
	if req.Status != "ACTIVE" && req.Status != "DISABLED" {
		return domain.Department{}, ErrInvalidDepartment
	}
	if creating && (req.Key == "" || len(req.Key) > 100 || !validDirectoryKey(req.Key)) {
		return domain.Department{}, ErrInvalidDepartment
	}
	return req, nil
}

func validDirectoryKey(value string) bool {
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func validateDepartmentHierarchy(items []domain.Department, candidate domain.Department, creating bool) error {
	byID := make(map[string]domain.Department, len(items)+1)
	for _, item := range items {
		byID[item.ID] = item
	}
	if !creating {
		if _, ok := byID[candidate.ID]; !ok {
			return sql.ErrNoRows
		}
		byID[candidate.ID] = candidate
	}
	if candidate.ParentID != "" {
		parent, ok := byID[candidate.ParentID]
		if !ok || parent.OrganizationID != candidate.OrganizationID || parent.ID == candidate.ID {
			return ErrInvalidDepartment
		}
		if candidate.Status == "ACTIVE" && parent.Status != "ACTIVE" {
			return ErrInvalidDepartment
		}
	}
	seen := map[string]struct{}{candidate.ID: {}}
	for parentID := candidate.ParentID; parentID != ""; {
		if _, exists := seen[parentID]; exists {
			return ErrInvalidDepartment
		}
		seen[parentID] = struct{}{}
		parent, ok := byID[parentID]
		if !ok {
			return ErrInvalidDepartment
		}
		parentID = parent.ParentID
	}
	if candidate.Status == "DISABLED" {
		for _, item := range byID {
			if item.ParentID == candidate.ID && item.Status == "ACTIVE" {
				return ErrInvalidDepartment
			}
		}
	}
	return nil
}

func (s *Service) ListRoles(ctx context.Context, orgID string) ([]domain.Role, error) {
	return s.repo.ListRoles(ctx, orgID)
}

func (s *Service) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *Service) UpdateRolePermissions(ctx context.Context, actor domain.Principal, orgID, roleKey string, permissionKeys []string) error {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return err
	}
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
	roles, err := s.repo.ListRoles(ctx, orgID)
	if err != nil {
		return err
	}
	var current []string
	found := false
	for _, role := range roles {
		if role.Key != roleKey {
			continue
		}
		found = true
		for _, permission := range role.Permissions {
			current = append(current, permission.Key)
		}
		break
	}
	if !found {
		return ErrInvalidRole
	}
	if err := enforcePermissionMutation(actor, roleKey, current, clean); err != nil {
		return err
	}
	return s.repo.ReplaceRolePermissions(ctx, orgID, roleKey, clean)
}

func (s *Service) UpdateUserRoles(ctx context.Context, actor domain.Principal, orgID, userID string, roleKeys []string) error {
	if err := authorizeGrantActor(actor, orgID); err != nil {
		return err
	}
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
	target, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if target.OrganizationID != orgID {
		return ErrGrantCeiling
	}
	if err := enforceRoleMutation(actor, target.Roles, clean); err != nil {
		return err
	}
	return s.repo.ReplaceUserRoles(ctx, orgID, userID, clean)
}

func authorizeGrantActor(actor domain.Principal, orgID string) error {
	if actor.Type != "USER" || actor.UserID == "" || actor.OrganizationID == "" || actor.OrganizationID != orgID {
		return ErrGrantCeiling
	}
	return nil
}

func enforceRoleMutation(actor domain.Principal, current, next []string) error {
	if contains(actor.Roles, "system_admin") {
		return nil
	}
	for _, key := range changedValues(current, next) {
		if !contains(actor.Roles, key) {
			return ErrGrantCeiling
		}
	}
	return nil
}

func enforcePermissionMutation(actor domain.Principal, roleKey string, current, next []string) error {
	if contains(actor.Roles, "system_admin") {
		return nil
	}
	if !contains(actor.Roles, roleKey) {
		return ErrGrantCeiling
	}
	for _, key := range changedValues(current, next) {
		if !contains(actor.Permissions, key) {
			return ErrGrantCeiling
		}
	}
	return nil
}

func changedValues(current, next []string) []string {
	before := make(map[string]struct{}, len(current))
	after := make(map[string]struct{}, len(next))
	for _, value := range current {
		before[value] = struct{}{}
	}
	for _, value := range next {
		after[value] = struct{}{}
	}
	changed := make([]string, 0)
	for value := range before {
		if _, ok := after[value]; !ok {
			changed = append(changed, value)
		}
	}
	for value := range after {
		if _, ok := before[value]; !ok {
			changed = append(changed, value)
		}
	}
	return changed
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
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
	if err := s.enforceOrganizationActive(ctx, orgID); err != nil {
		return err
	}
	policy, err := s.resolveSecurityPolicy(ctx, orgID)
	if err != nil {
		return err
	}
	if err := policy.passwordPolicy.Validate(next); err != nil {
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
	if policy.history > 0 {
		history, err := s.repo.PasswordHistory(ctx, userID, policy.history)
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
	return s.repo.AdminResetPassword(ctx, orgID, userID, currentHash, nextHash)
}
