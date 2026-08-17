package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sevoniva-labs/forge/internal/app/audit"
	appidentity "github.com/sevoniva-labs/forge/internal/app/identity"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
	"github.com/sevoniva-labs/forge/internal/platform/cache"
	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/database"
	"github.com/sevoniva-labs/forge/internal/platform/discovery"
	"github.com/sevoniva-labs/forge/internal/platform/health"
	"github.com/sevoniva-labs/forge/internal/platform/httpx"
	"github.com/sevoniva-labs/forge/internal/platform/messaging"
	"github.com/sevoniva-labs/forge/internal/platform/metrics"
	"github.com/sevoniva-labs/forge/internal/platform/ratelimit"
	"github.com/sevoniva-labs/forge/internal/platform/search"
	appcrypto "github.com/sevoniva-labs/forge/internal/platform/security/crypto"
	"github.com/sevoniva-labs/forge/internal/platform/storage"
)

const sessionCookie = "forge_session"
const csrfCookie = "forge_csrf"

type Dependencies struct {
	Config    config.Config
	Log       *slog.Logger
	DB        *database.DB
	Discovery discovery.Registry
	Cache     cache.Cache
	Messaging messaging.Bus
	Search    search.Engine
	Storage   storage.Store
	Crypto    appcrypto.Provider
	Identity  *appidentity.Service
	Audit     *audit.Writer
	Metrics   *metrics.Metrics
	Version   string
}

type Server struct {
	cfg           config.Config
	log           *slog.Logger
	db            *database.DB
	discovery     discovery.Registry
	cache         cache.Cache
	messaging     messaging.Bus
	search        search.Engine
	storage       storage.Store
	crypto        appcrypto.Provider
	identity      *appidentity.Service
	audit         *audit.Writer
	metrics       *metrics.Metrics
	version       string
	mux           http.Handler
	limiter       *ratelimit.Limiter
	secureCookies bool
	trusted       []*net.IPNet
}

func New(d Dependencies) *Server {
	s := &Server{cfg: d.Config, log: d.Log, db: d.DB, discovery: d.Discovery, cache: d.Cache, messaging: d.Messaging, search: d.Search, storage: d.Storage, crypto: d.Crypto, identity: d.Identity, audit: d.Audit, metrics: d.Metrics, version: d.Version, limiter: ratelimit.New(d.Cache), secureCookies: d.Config.Security.SecureCookies}
	s.trusted = parseTrusted(d.Config.Security.TrustedProxies)
	r := chi.NewRouter()
	r.Use(requestID, tracing, recoverer(s.log, strings.EqualFold(s.cfg.App.Environment, "development")), securityHeaders(s.secureCookies), cors(s.cfg.Security.AllowedOrigins), bodyLimit(s.cfg.Server.MaxBodyBytes), accessLog(s.log, s.metrics), gzipJSON)
	r.Get("/api/v1/system/health", s.health)
	r.Get("/api/v1/system/ready", s.ready)
	r.Post("/api/v1/auth/login", loginRateLimit(s.limiter, s.clientIP, s.login))
	r.Group(func(r chi.Router) {
		r.Use(s.auth)
		r.Get("/api/v1/me", s.me)
		r.Get("/api/v1/system/info", s.info)
		r.Post("/api/v1/auth/logout", s.logout)
		r.Patch("/api/v1/auth/password", s.changePassword)
		r.Get("/api/v1/api-tokens", s.listAPITokens)
		r.Post("/api/v1/api-tokens", s.createAPIToken)
		r.Delete("/api/v1/api-tokens/{tokenID}", s.revokeAPIToken)
		r.Route("/api/v1/admin", func(r chi.Router) {
			r.With(requirePermissions("system.organization.read")).Get("/organization", s.getOrganization)
			r.With(requirePermissions("system.organization.manage")).Patch("/organization", s.updateOrganization)
			r.With(requirePermissions("system.config.read")).Get("/security-config", s.getSecurityConfig)
			r.With(requirePermissions("system.security.manage")).Put("/security-config", s.updateSecurityConfig)
			r.With(requirePermissions("system.role.read")).Get("/roles", s.listRoles)
			r.With(requirePermissions("system.role.read")).Get("/permissions", s.listPermissions)
			r.With(requirePermissions("system.role.manage")).Put("/roles/{roleKey}/permissions", s.updateRolePermissions)
			r.With(requirePermissions("system.user.read")).Get("/users", s.listUsers)
			r.With(requirePermissions("system.user.create")).Post("/users", s.createUser)
			r.With(requirePermissions("system.user.role.manage")).Patch("/users/{userID}/roles", s.updateUserRoles)
			r.With(requirePermissions("system.user.update")).Patch("/users/{userID}/status", s.updateUserStatus)
			r.With(requirePermissions("system.user.update")).Post("/users/{userID}/unlock", s.unlockUser)
			r.With(requirePermissions("system.user.update")).Post("/users/{userID}/reset-password", s.resetUserPassword)
			r.With(requirePermissions("system.session.read")).Get("/sessions", s.listSessions)
			r.With(requirePermissions("system.session.revoke")).Delete("/sessions/{sessionID}", s.revokeSession)
			r.With(requirePermissions("system.audit.read")).Get("/audit-logs", s.listAuditLogs)
			r.With(requirePermissions("system.audit.export")).Get("/audit-logs/export", s.exportAuditLogs)
		})
	})
	if s.metrics != nil && s.cfg.Observability.MetricsEnabled {
		r.Handle(s.cfg.Observability.MetricsPath, s.metrics.Handler())
	}
	if s.cfg.Observability.PprofEnabled && !s.cfg.Compliance.DisableDebugEndpoints {
		r.Get("/debug/pprof/", pprof.Index)
		r.Get("/debug/pprof/cmdline", pprof.Cmdline)
		r.Get("/debug/pprof/profile", pprof.Profile)
		r.Get("/debug/pprof/symbol", pprof.Symbol)
		r.Post("/debug/pprof/symbol", pprof.Symbol)
		r.Get("/debug/pprof/trace", pprof.Trace)
		r.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
		r.Handle("/debug/pprof/block", pprof.Handler("block"))
		r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		r.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
		r.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	}
	if s.cfg.Server.WebDir != "" {
		r.NotFound(spa(s.cfg.Server.WebDir))
	}
	s.mux = r
	return s
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.Success(w, 200, map[string]any{"status": "UP", "service": s.cfg.App.Name, "version": s.version, "time": time.Now().UTC()}, RequestID(r), TraceID(r))
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	checks := []health.Check{{Name: "database", Provider: s.cfg.Database.Provider, Ping: s.db.PingContext}, {Name: "cache", Provider: s.cache.Provider(), Ping: s.cache.Ping}, {Name: "messaging", Provider: s.messaging.Provider(), Ping: s.messaging.Ping}, {Name: "search", Provider: s.search.Provider(), Ping: s.search.Ping}, {Name: "storage", Provider: s.storage.Provider(), Ping: s.storage.Ping}}
	if s.discovery != nil {
		checks = append(checks, health.Check{Name: "discovery", Provider: s.discovery.Provider(), Ping: s.discovery.Ping})
	}
	res := health.Run(r.Context(), checks)
	if !strings.EqualFold(s.cfg.App.Environment, "development") {
		for i := range res {
			res[i].Error = ""
			res[i].Provider = ""
		}
	}
	status := "UP"
	code := 200
	for _, x := range res {
		if x.Status != "UP" {
			status = "DOWN"
			code = 503
		}
	}
	httpx.Success(w, code, map[string]any{"status": status, "checks": res}, RequestID(r), TraceID(r))
}
func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	httpx.Success(w, 200, map[string]any{"application": s.cfg.App.Name, "environment": s.cfg.App.Environment, "version": s.version, "providers": map[string]string{"database": s.cfg.Database.Provider, "cache": s.cache.Provider(), "messaging": s.messaging.Provider(), "search": s.search.Provider(), "storage": s.storage.Provider(), "crypto": s.crypto.Name(), "discovery": providerName(s.discovery), "remote_config": s.cfg.RemoteConfig.Provider}, "compliance_profile": s.cfg.Compliance.Profile}, RequestID(r), TraceID(r))
}

type loginRequest struct {
	Organization string `json:"organization"`
	LoginName    string `json:"login_name"`
	Password     string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, 64<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	orgID, err := s.organizationID(r.Context(), req.Organization)
	if err != nil {
		httpx.Error(w, 401, "INVALID_CREDENTIALS", "用户名或密码错误", RequestID(r), TraceID(r))
		if s.audit != nil {
			_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: orgID, ActorName: req.LoginName, Action: "auth.login", Result: "FAILED", ClientIP: s.clientIP(r)})
		}
		return
	}
	p, token, csrf, expires, err := s.identity.Login(r.Context(), orgID, req.LoginName, req.Password, s.clientIP(r), limitString(r.UserAgent(), 512))
	if err != nil {
		code, msg := 401, "用户名或密码错误"
		if errors.Is(err, appidentity.ErrLocked) {
			code = 423
			msg = "账号已锁定，请稍后再试"
		}
		httpx.Error(w, code, "LOGIN_FAILED", msg, RequestID(r), TraceID(r))
		if s.audit != nil {
			_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: orgID, ActorName: req.LoginName, Action: "auth.login", Result: "FAILED", ClientIP: s.clientIP(r)})
		}
		return
	}
	setCookies(w, token, csrf, expires, s.secureCookies, s.cfg.Security.SameSite)
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "auth.login", Result: "SUCCESS", ClientIP: s.clientIP(r)})
	}
	httpx.Success(w, 200, p, RequestID(r), TraceID(r))
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	if c, e := r.Cookie(sessionCookie); e == nil {
		_ = s.identity.Logout(r.Context(), c.Value)
	}
	clearCookies(w, s.secureCookies)
	if p != nil && s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "auth.logout", ClientIP: s.clientIP(r)})
	}
	httpx.Success(w, http.StatusOK, nil, RequestID(r), TraceID(r))
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	httpx.Success(w, 200, Principal(r), RequestID(r), TraceID(r))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	var req changePasswordRequest
	if err := httpx.DecodeJSON(w, r, 64<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	if err := s.identity.ChangePassword(r.Context(), p.UserID, p.SessionID, req.CurrentPassword, req.NewPassword); err != nil {
		code, msg := "PASSWORD_CHANGE_FAILED", "密码修改失败"
		if errors.Is(err, appidentity.ErrInvalidCredentials) {
			code, msg = "CURRENT_PASSWORD_INVALID", "当前密码错误"
		} else if errors.Is(err, appidentity.ErrPasswordPolicy) {
			code, msg = "PASSWORD_POLICY_VIOLATION", "新密码不符合安全策略"
		} else if errors.Is(err, appidentity.ErrPasswordReused) {
			code, msg = "PASSWORD_REUSED", "新密码不能与近期使用过的密码相同"
		}
		httpx.Error(w, 400, code, msg, RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "auth.password.change", ClientIP: s.clientIP(r)})
	}
	httpx.Success(w, http.StatusOK, nil, RequestID(r), TraceID(r))
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	items, err := s.identity.ListUsers(r.Context(), p.OrganizationID)
	if err != nil {
		httpx.Error(w, 500, "INTERNAL", "查询用户失败", RequestID(r), TraceID(r))
		return
	}
	httpx.Success(w, 200, map[string]any{"items": items}, RequestID(r), TraceID(r))
}

type createUserRequest struct {
	LoginName   string   `json:"login_name"`
	DisplayName string   `json:"display_name"`
	Password    string   `json:"password"`
	Roles       []string `json:"roles"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	var req createUserRequest
	if err := httpx.DecodeJSON(w, r, 128<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	if !p.HasPermission("system.user.role.manage") {
		for _, role := range req.Roles {
			if strings.TrimSpace(role) != "" && strings.TrimSpace(role) != "user" {
				httpx.Error(w, http.StatusForbidden, "PERMISSION_DENIED", "无权限分配特权角色", RequestID(r), TraceID(r))
				return
			}
		}
	}
	u, err := s.identity.CreateUser(r.Context(), *p, p.OrganizationID, req.LoginName, req.DisplayName, req.Password, req.Roles)
	if err != nil {
		code, msg, status := "CREATE_USER_FAILED", "创建用户失败", http.StatusBadRequest
		switch {
		case errors.Is(err, appidentity.ErrInvalidRole):
			code, msg = "INVALID_ROLE", "角色不合法"
		case errors.Is(err, appidentity.ErrInvalidLoginName):
			code, msg = "INVALID_LOGIN_NAME", "登录名不合法"
		case errors.Is(err, appidentity.ErrPasswordPolicy):
			code, msg = "PASSWORD_POLICY_VIOLATION", "初始密码不符合安全策略"
		case errors.Is(err, appidentity.ErrGrantCeiling):
			code, msg, status = "GRANT_CEILING_EXCEEDED", "不能授予超出当前账号权限范围的角色", http.StatusForbidden
		case strings.Contains(strings.ToLower(err.Error()), "duplicate"), strings.Contains(strings.ToLower(err.Error()), "unique"):
			code, msg, status = "LOGIN_NAME_CONFLICT", "登录名已存在", http.StatusConflict
		default:
			s.log.Error("create user failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
			status = http.StatusInternalServerError
		}
		httpx.Error(w, status, code, msg, RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "user.create", ResourceType: "user", ResourceID: u.ID, ClientIP: s.clientIP(r), Details: map[string]any{"login_name": u.LoginName, "roles": u.Roles}})
	}
	httpx.Success(w, 201, u, RequestID(r), TraceID(r))
}

func (s *Server) getOrganization(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	item, err := s.identity.Organization(r.Context(), p.OrganizationID)
	if err != nil {
		httpx.Error(w, 500, "INTERNAL", "查询组织信息失败", RequestID(r), TraceID(r))
		return
	}
	httpx.Success(w, 200, item, RequestID(r), TraceID(r))
}

type updateOrganizationRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	Status            string `json:"status"`
	MaxUsers          int    `json:"max_users"`
	MaxActiveSessions int    `json:"max_active_sessions"`
}

func (s *Server) updateOrganization(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	var req updateOrganizationRequest
	if err := httpx.DecodeJSON(w, r, 128<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	item, err := s.identity.UpdateOrganization(r.Context(), p.OrganizationID, domain.Organization{
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		MaxUsers:    req.MaxUsers,
		MaxSessions: req.MaxActiveSessions,
	})
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "invalid organization"):
			httpx.Error(w, 400, "INVALID_ORGANIZATION", "组织参数不合法", RequestID(r), TraceID(r))
		default:
			s.log.Error("update organization failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
			httpx.Error(w, 500, "INTERNAL", "更新组织信息失败", RequestID(r), TraceID(r))
		}
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "organization.update", ResourceType: "organization", ResourceID: item.ID, ClientIP: s.clientIP(r), Details: map[string]any{
			"name":                req.Name,
			"description":         req.Description,
			"status":              req.Status,
			"max_users":           req.MaxUsers,
			"max_active_sessions": req.MaxActiveSessions,
		}})
	}
	httpx.Success(w, 200, item, RequestID(r), TraceID(r))
}

func (s *Server) getSecurityConfig(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	policy, err := s.identity.SecurityPolicy(r.Context(), p.OrganizationID)
	if err != nil {
		s.log.Error("get security config failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
		httpx.Error(w, 500, "INTERNAL", "查询安全配置失败", RequestID(r), TraceID(r))
		return
	}
	httpx.Success(w, 200, policy, RequestID(r), TraceID(r))
}

type updateSecurityConfigRequest struct {
	PasswordMinLength        int   `json:"password_min_length"`
	PasswordRequireUpper     bool  `json:"password_require_upper"`
	PasswordRequireLower     bool  `json:"password_require_lower"`
	PasswordRequireDigit     bool  `json:"password_require_digit"`
	PasswordRequireSymbol    bool  `json:"password_require_symbol"`
	PasswordHistory          int   `json:"password_history"`
	PasswordMaxAgeDays       int   `json:"password_max_age_days"`
	LoginMaxFailures         int   `json:"login_max_failures"`
	LoginLockDurationSeconds int64 `json:"login_lock_duration_seconds"`
	SessionTTLSeconds        int64 `json:"session_ttl_seconds"`
	MaxConcurrentSessions    int   `json:"max_active_sessions"`
}

func (s *Server) updateSecurityConfig(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	var req updateSecurityConfigRequest
	if err := httpx.DecodeJSON(w, r, 128<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	policy, err := s.identity.UpdateSecurityPolicy(r.Context(), p.OrganizationID, p.UserID, domain.SecurityPolicy{
		PasswordMinLength:        req.PasswordMinLength,
		PasswordRequireUpper:     req.PasswordRequireUpper,
		PasswordRequireLower:     req.PasswordRequireLower,
		PasswordRequireDigit:     req.PasswordRequireDigit,
		PasswordRequireSymbol:    req.PasswordRequireSymbol,
		PasswordHistory:          req.PasswordHistory,
		PasswordMaxAgeDays:       req.PasswordMaxAgeDays,
		LoginMaxFailures:         req.LoginMaxFailures,
		LoginLockDurationSeconds: req.LoginLockDurationSeconds,
		SessionTTLSeconds:        req.SessionTTLSeconds,
		MaxConcurrentSessions:    req.MaxConcurrentSessions,
	})
	if err != nil {
		if errors.Is(err, appidentity.ErrInvalidSecurityPolicy) {
			httpx.Error(w, 400, "INVALID_SECURITY_CONFIG", "安全配置参数不合法", RequestID(r), TraceID(r))
			return
		}
		s.log.Error("update security config failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
		httpx.Error(w, 500, "INTERNAL", "更新安全配置失败", RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "security.config.update", ResourceType: "security", ResourceID: "policy", ClientIP: s.clientIP(r), Details: map[string]any{
			"password_min_length":         req.PasswordMinLength,
			"password_require_upper":      req.PasswordRequireUpper,
			"password_require_lower":      req.PasswordRequireLower,
			"password_require_digit":      req.PasswordRequireDigit,
			"password_require_symbol":     req.PasswordRequireSymbol,
			"password_history":            req.PasswordHistory,
			"password_max_age_days":       req.PasswordMaxAgeDays,
			"login_max_failures":          req.LoginMaxFailures,
			"login_lock_duration_seconds": req.LoginLockDurationSeconds,
			"session_ttl_seconds":         req.SessionTTLSeconds,
			"max_active_sessions":         req.MaxConcurrentSessions,
		}})
	}
	httpx.Success(w, 200, policy, RequestID(r), TraceID(r))
}

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	items, err := s.identity.ListRoles(r.Context(), p.OrganizationID)
	if err != nil {
		httpx.Error(w, 500, "INTERNAL", "查询角色失败", RequestID(r), TraceID(r))
		return
	}
	httpx.Success(w, 200, map[string]any{"items": items}, RequestID(r), TraceID(r))
}

func (s *Server) listPermissions(w http.ResponseWriter, r *http.Request) {
	items, err := s.identity.ListPermissions(r.Context())
	if err != nil {
		httpx.Error(w, 500, "INTERNAL", "查询权限失败", RequestID(r), TraceID(r))
		return
	}
	httpx.Success(w, 200, map[string]any{"items": items}, RequestID(r), TraceID(r))
}

type updateRolePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

func (s *Server) updateRolePermissions(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	roleKey := chi.URLParam(r, "roleKey")
	var req updateRolePermissionsRequest
	if err := httpx.DecodeJSON(w, r, 128<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	if err := s.identity.UpdateRolePermissions(r.Context(), *p, p.OrganizationID, roleKey, req.Permissions); err != nil {
		code, msg, status := "ROLE_PERMISSION_UPDATE_FAILED", "角色权限更新失败", http.StatusBadRequest
		if errors.Is(err, appidentity.ErrInvalidRole) {
			code, msg = "INVALID_ROLE", "角色或权限不合法"
		} else if errors.Is(err, appidentity.ErrGrantCeiling) {
			code, msg, status = "GRANT_CEILING_EXCEEDED", "不能修改超出当前账号权限范围的角色权限", http.StatusForbidden
		}
		httpx.Error(w, status, code, msg, RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "role.permissions.update", ResourceType: "role", ResourceID: roleKey, ClientIP: s.clientIP(r), Details: map[string]any{"permissions": req.Permissions}})
	}
	httpx.Success(w, 200, nil, RequestID(r), TraceID(r))
}

type updateUserRolesRequest struct {
	Roles []string `json:"roles"`
}

func (s *Server) updateUserRoles(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	userID := chi.URLParam(r, "userID")
	if userID == p.UserID {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "不能通过管理接口修改当前账号角色", RequestID(r), TraceID(r))
		return
	}
	var req updateUserRolesRequest
	if err := httpx.DecodeJSON(w, r, 64<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	if err := s.identity.UpdateUserRoles(r.Context(), *p, p.OrganizationID, userID, req.Roles); err != nil {
		code, msg, status := "USER_ROLE_UPDATE_FAILED", "用户角色更新失败", http.StatusBadRequest
		if errors.Is(err, appidentity.ErrInvalidRole) {
			code, msg = "INVALID_ROLE", "角色不合法"
		} else if errors.Is(err, appidentity.ErrLastSystemAdmin) {
			code, msg, status = "LAST_SYSTEM_ADMIN", "至少保留一个启用状态的系统管理员", http.StatusConflict
		} else if errors.Is(err, sql.ErrNoRows) {
			code, msg, status = "NOT_FOUND", "用户不存在", http.StatusNotFound
		} else if errors.Is(err, appidentity.ErrGrantCeiling) {
			code, msg, status = "GRANT_CEILING_EXCEEDED", "不能修改超出当前账号权限范围的角色", http.StatusForbidden
		}
		httpx.Error(w, status, code, msg, RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "user.roles.update", ResourceType: "user", ResourceID: userID, ClientIP: s.clientIP(r), Details: map[string]any{"roles": req.Roles}})
	}
	httpx.Success(w, 200, nil, RequestID(r), TraceID(r))
}

type updateUserStatusRequest struct {
	Status string `json:"status"`
}

func (s *Server) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	userID := chi.URLParam(r, "userID")
	if userID == p.UserID {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "不能通过管理接口修改当前账号状态", RequestID(r), TraceID(r))
		return
	}
	var req updateUserStatusRequest
	if err := httpx.DecodeJSON(w, r, 32<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	if err := s.identity.SetUserStatus(r.Context(), p.OrganizationID, userID, req.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "用户不存在", RequestID(r), TraceID(r))
			return
		}
		if errors.Is(err, appidentity.ErrLastSystemAdmin) {
			httpx.Error(w, http.StatusConflict, "LAST_SYSTEM_ADMIN", "至少保留一个启用状态的系统管理员", RequestID(r), TraceID(r))
			return
		}
		httpx.Error(w, http.StatusBadRequest, "USER_STATUS_UPDATE_FAILED", "用户状态更新失败", RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "user.status.update", ResourceType: "user", ResourceID: userID, ClientIP: s.clientIP(r), Details: map[string]any{"status": req.Status}})
	}
	httpx.Success(w, 200, nil, RequestID(r), TraceID(r))
}

func (s *Server) unlockUser(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	userID := chi.URLParam(r, "userID")
	if err := s.identity.UnlockUser(r.Context(), p.OrganizationID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "用户不存在", RequestID(r), TraceID(r))
			return
		}
		httpx.Error(w, http.StatusBadRequest, "USER_UNLOCK_FAILED", "账号解锁失败", RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "user.unlock", ResourceType: "user", ResourceID: userID, ClientIP: s.clientIP(r)})
	}
	httpx.Success(w, 200, nil, RequestID(r), TraceID(r))
}

type resetUserPasswordRequest struct {
	Password string `json:"password"`
}

func (s *Server) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	userID := chi.URLParam(r, "userID")
	if userID == p.UserID {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "当前账号请在账号安全页面修改密码", RequestID(r), TraceID(r))
		return
	}
	var req resetUserPasswordRequest
	if err := httpx.DecodeJSON(w, r, 32<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	if err := s.identity.AdminResetPassword(r.Context(), p.OrganizationID, userID, req.Password); err != nil {
		code, msg, status := "PASSWORD_RESET_FAILED", "密码重置失败", http.StatusBadRequest
		if errors.Is(err, appidentity.ErrPasswordPolicy) {
			code, msg = "PASSWORD_POLICY_VIOLATION", "新密码不符合安全策略"
		} else if errors.Is(err, appidentity.ErrPasswordReused) {
			code, msg = "PASSWORD_REUSED", "新密码不能与当前密码相同"
		} else if errors.Is(err, sql.ErrNoRows) {
			code, msg, status = "NOT_FOUND", "用户不存在", http.StatusNotFound
		} else if errors.Is(err, appidentity.ErrPasswordStateChanged) {
			code, msg, status = "PASSWORD_STATE_CHANGED", "密码已被其他管理员更新，请重试", http.StatusConflict
		}
		httpx.Error(w, status, code, msg, RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "user.password.reset", ResourceType: "user", ResourceID: userID, ClientIP: s.clientIP(r)})
	}
	httpx.Success(w, 200, nil, RequestID(r), TraceID(r))
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	items, err := s.identity.ListSessions(r.Context(), p.OrganizationID, p.SessionID)
	if err != nil {
		httpx.Error(w, 500, "INTERNAL", "查询在线会话失败", RequestID(r), TraceID(r))
		return
	}
	httpx.Success(w, 200, map[string]any{"items": items}, RequestID(r), TraceID(r))
}

func (s *Server) revokeSession(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	sessionID := chi.URLParam(r, "sessionID")
	if err := s.identity.RevokeSession(r.Context(), p.OrganizationID, sessionID, p.SessionID); err != nil {
		status, code, msg := http.StatusBadRequest, "SESSION_REVOKE_FAILED", "会话下线失败"
		if errors.Is(err, sql.ErrNoRows) {
			status, code, msg = http.StatusNotFound, "NOT_FOUND", "会话不存在"
		}
		httpx.Error(w, status, code, msg, RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "session.revoke", ResourceType: "session", ResourceID: sessionID, ClientIP: s.clientIP(r)})
	}
	httpx.Success(w, 200, nil, RequestID(r), TraceID(r))
}

func (s *Server) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	items, err := s.audit.List(r.Context(), p.OrganizationID, 200)
	if err != nil {
		s.log.Error("list audit logs failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
		httpx.Error(w, 500, "INTERNAL", "查询审计日志失败", RequestID(r), TraceID(r))
		return
	}
	httpx.Success(w, 200, map[string]any{"items": items}, RequestID(r), TraceID(r))
}

func (s *Server) exportAuditLogs(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	format := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		httpx.Error(w, 400, "INVALID_REQUEST", "export format only supports json or csv", RequestID(r), TraceID(r))
		return
	}

	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			httpx.Error(w, 400, "INVALID_REQUEST", "limit must be positive integer", RequestID(r), TraceID(r))
			return
		}
		limit = v
	}

	items, err := s.audit.List(r.Context(), p.OrganizationID, limit)
	if err != nil {
		s.log.Error("export audit logs failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
		httpx.Error(w, 500, "INTERNAL", "导出审计日志失败", RequestID(r), TraceID(r))
		return
	}
	filename := "audit-logs-" + time.Now().UTC().Format("20060102-150405") + "." + format
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		var buf bytes.Buffer
		cw := csv.NewWriter(&buf)
		_ = cw.Write([]string{
			"id",
			"occurred_at",
			"request_id",
			"organization_id",
			"actor_id",
			"actor_name",
			"action",
			"resource_type",
			"resource_id",
			"result",
			"client_ip",
			"details",
		})
		for _, item := range items {
			details, _ := json.Marshal(item.Details)
			_ = cw.Write([]string{
				item.ID,
				item.OccurredAt.UTC().Format(time.RFC3339Nano),
				item.RequestID,
				item.OrganizationID,
				item.ActorID,
				item.ActorName,
				item.Action,
				item.ResourceType,
				item.ResourceID,
				item.Result,
				item.ClientIP,
				string(details),
			})
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			s.log.Error("write csv audit logs failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
			httpx.Error(w, 500, "INTERNAL", "导出审计日志失败", RequestID(r), TraceID(r))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	default:
		data, err := json.Marshal(items)
		if err != nil {
			s.log.Error("encode json audit logs failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
			httpx.Error(w, 500, "INTERNAL", "导出审计日志失败", RequestID(r), TraceID(r))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}

	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "audit.export", ResourceType: "audit_log", ClientIP: s.clientIP(r), Details: map[string]any{"format": format, "limit": limit}})
	}
}

func (s *Server) organizationID(ctx context.Context, key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		key = "default"
	}
	var id string
	err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT id FROM organizations WHERE org_key=?`), key).Scan(&id)
	return id, err
}

func setCookies(w http.ResponseWriter, session, csrf string, expires time.Time, secure bool, sameSite string) {
	site := parseSameSite(sameSite)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: session, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: secure, SameSite: site})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: csrf, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), HttpOnly: false, Secure: secure, SameSite: site})
}
func clearCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0), HttpOnly: name == sessionCookie, Secure: secure, SameSite: http.SameSiteLaxMode})
	}
}
func parseSameSite(v string) http.SameSite {
	switch strings.ToLower(v) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
func parseTrusted(xs []string) []*net.IPNet {
	var out []*net.IPNet
	for _, raw := range xs {
		x := strings.TrimSpace(raw)
		if x == "" {
			continue
		}
		if !strings.Contains(x, "/") {
			ip := net.ParseIP(x)
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				x += "/32"
			} else {
				x += "/128"
			}
		}
		if _, n, e := net.ParseCIDR(x); e == nil {
			out = append(out, n)
		}
	}
	return out
}
func (s *Server) clientIP(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	trusted := false
	for _, n := range s.trusted {
		if ip != nil && n.Contains(ip) {
			trusted = true
			break
		}
	}
	if trusted {
		if x := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(x) != nil {
			return x
		}
	}
	return host
}
func limitString(v string, n int) string {
	if len(v) > n {
		return v[:n]
	}
	return v
}
func spa(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		p := filepath.Join(dir, clean)
		if clean != "." {
			if st, e := os.Stat(p); e == nil && !st.IsDir() {
				http.ServeFile(w, r, p)
				return
			}
		}
		index := filepath.Join(dir, "index.html")
		if _, e := os.Stat(index); e != nil {
			httpx.Error(w, 404, "NOT_FOUND", "资源不存在", RequestID(r), TraceID(r))
			return
		}
		http.ServeFile(w, r, index)
	}
}

type createAPITokenRequest struct {
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	ExpiresDays int      `json:"expires_days"`
}

func (s *Server) listAPITokens(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	items, err := s.identity.ListAPITokens(r.Context(), *p)
	if err != nil {
		if errors.Is(err, appidentity.ErrInteractiveSessionRequired) {
			httpx.Error(w, http.StatusForbidden, "INTERACTIVE_SESSION_REQUIRED", "API Token 管理仅允许交互式用户会话", RequestID(r), TraceID(r))
			return
		}
		httpx.Error(w, 500, "INTERNAL", "查询 API Token 失败", RequestID(r), TraceID(r))
		return
	}
	httpx.Success(w, 200, map[string]any{"items": items}, RequestID(r), TraceID(r))
}
func (s *Server) createAPIToken(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	var req createAPITokenRequest
	if err := httpx.DecodeJSON(w, r, 64<<10, &req); err != nil {
		httpx.Error(w, 400, "INVALID_JSON", "请求格式错误", RequestID(r), TraceID(r))
		return
	}
	days := req.ExpiresDays
	if days == 0 {
		days = 90
	}
	t, raw, err := s.identity.CreateAPIToken(r.Context(), *p, req.Name, req.Scopes, time.Duration(days)*24*time.Hour)
	if err != nil {
		if errors.Is(err, appidentity.ErrInteractiveSessionRequired) {
			httpx.Error(w, http.StatusForbidden, "INTERACTIVE_SESSION_REQUIRED", "API Token 管理仅允许交互式用户会话", RequestID(r), TraceID(r))
			return
		}
		httpx.Error(w, 400, "INVALID_REQUEST", "API Token 创建失败", RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "security.api_token.create", ResourceType: "api_token", ResourceID: t.ID, ClientIP: s.clientIP(r), Details: map[string]any{"name": t.Name, "scopes": t.Scopes}})
	}
	httpx.Success(w, http.StatusCreated, map[string]any{"token": t, "secret": raw, "warning": "secret 仅本次返回，请立即保存"}, RequestID(r), TraceID(r))
}
func (s *Server) revokeAPIToken(w http.ResponseWriter, r *http.Request) {
	p := Principal(r)
	tokenID := chi.URLParam(r, "tokenID")
	if err := s.identity.RevokeAPIToken(r.Context(), *p, tokenID); err != nil {
		if errors.Is(err, appidentity.ErrInteractiveSessionRequired) {
			httpx.Error(w, http.StatusForbidden, "INTERACTIVE_SESSION_REQUIRED", "API Token 管理仅允许交互式用户会话", RequestID(r), TraceID(r))
			return
		}
		httpx.Error(w, 404, "NOT_FOUND", "API Token 不存在", RequestID(r), TraceID(r))
		return
	}
	if s.audit != nil {
		_ = s.audit.Write(r.Context(), audit.Event{RequestID: RequestID(r), OrganizationID: p.OrganizationID, ActorID: p.UserID, ActorName: p.LoginName, Action: "security.api_token.revoke", ResourceType: "api_token", ResourceID: tokenID, ClientIP: s.clientIP(r)})
	}
	httpx.Success(w, 200, nil, RequestID(r), TraceID(r))
}

func providerName(r discovery.Registry) string {
	if r == nil {
		return "disabled"
	}
	return r.Provider()
}
