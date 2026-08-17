package httpapi

import (
	"compress/gzip"
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/sevoniva-labs/forge/internal/platform/httpx"
	"github.com/sevoniva-labs/forge/internal/platform/metrics"
	"github.com/sevoniva-labs/forge/internal/platform/ratelimit"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusWriter) Write(b []byte) (int, error) {
	n, e := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, e
}

type gzipWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) { return w.writer.Write(b) }

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" || len(id) > 80 {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("forge/http").Start(r.Context(), "HTTP "+r.Method)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func recoverer(log *slog.Logger, includeStack bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if x := recover(); x != nil {
					attrs := []any{"request_id", RequestID(r), "trace_id", TraceID(r)}
					if includeStack {
						attrs = append(attrs, "panic", x, "stack", string(debug.Stack()))
					}
					log.Error("panic recovered", attrs...)
					httpx.Error(w, 500, "INTERNAL", "服务器内部错误", RequestID(r), TraceID(r))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func securityHeaders(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
			if secure {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func cors(allowed []string) func(http.Handler) http.Handler {
	set := map[string]struct{}{}
	for _, v := range allowed {
		set[v] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := set[origin]; ok && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, X-Request-ID, Authorization")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func gzipJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz := gzip.NewWriter(w)
		defer gz.Close()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		next.ServeHTTP(gzipWriter{ResponseWriter: w, writer: gz}, r)
	})
}

func accessLog(log *slog.Logger, m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			if m != nil {
				finish := m.Begin(r.Method)
				defer finish()
			}
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			route := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil {
				if pattern := rc.RoutePattern(); pattern != "" {
					route = pattern
				}
			}
			span := trace.SpanFromContext(r.Context())
			span.SetName(r.Method + " " + route)
			span.SetAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", sw.status),
			)

			attrs := []any{"method", r.Method, "path", r.URL.Path, "route", route, "status", sw.status, "duration_ms", time.Since(start).Milliseconds(), "bytes", sw.bytes, "request_id", RequestID(r)}
			if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
				attrs = append(attrs, "trace_id", sc.TraceID().String())
			}
			if p := Principal(r); p != nil {
				attrs = append(attrs, "principal_type", p.Type, "user_id", p.UserID, "organization_id", p.OrganizationID)
			}
			log.Info("http", attrs...)
			if m != nil {
				m.Observe(r.Method, route, sw.status, sw.bytes, start)
			}
		})
	}
}

func bodyLimit(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if max > 0 && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, max)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Service/API identities use Bearer tokens and are not subject to browser
		// CSRF checks. Token scopes are enforced by the same permission middleware.
		if h := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(h, "Bearer ") {
			raw := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			p, err := s.identity.AuthenticateAPIToken(r.Context(), raw)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "API Token 无效或已过期", RequestID(r), TraceID(r))
				return
			}
			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), p)))
			return
		}

		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			httpx.Error(w, 401, "UNAUTHENTICATED", "未认证", RequestID(r), TraceID(r))
			return
		}
		p, err := s.identity.Authenticate(r.Context(), c.Value)
		if err != nil {
			clearCookies(w, s.secureCookies)
			httpx.Error(w, 401, "UNAUTHENTICATED", "会话已失效", RequestID(r), TraceID(r))
			return
		}
		if p.MustChangePassword && !allowedBeforePasswordChange(r.Method, r.URL.Path) {
			httpx.Error(w, 403, "PASSWORD_CHANGE_REQUIRED", "首次登录必须先修改密码", RequestID(r), TraceID(r))
			return
		}
		if isWrite(r.Method) {
			csrf := r.Header.Get("X-CSRF-Token")
			cc, _ := r.Cookie(csrfCookie)
			if csrf == "" || cc == nil || subtle.ConstantTimeCompare([]byte(csrf), []byte(cc.Value)) != 1 {
				httpx.Error(w, 403, "CSRF_MISMATCH", "CSRF 校验失败", RequestID(r), TraceID(r))
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), p)))
	})
}

func allowedBeforePasswordChange(method, path string) bool {
	return (method == http.MethodGet && path == "/api/v1/me") ||
		(method == http.MethodPatch && path == "/api/v1/auth/password") ||
		(method == http.MethodPost && path == "/api/v1/auth/logout")
}

func requireRoles(keys ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := Principal(r)
			if p == nil {
				httpx.Error(w, 401, "UNAUTHENTICATED", "未认证", RequestID(r), TraceID(r))
				return
			}
			if !p.HasRole(keys...) {
				httpx.Error(w, 403, "PERMISSION_DENIED", "无权限执行此操作", RequestID(r), TraceID(r))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requirePermissions(keys ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := Principal(r)
			if p == nil {
				httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "未认证", RequestID(r), TraceID(r))
				return
			}
			for _, key := range keys {
				if !p.HasPermission(key) {
					httpx.Error(w, http.StatusForbidden, "PERMISSION_DENIED", "无权限执行此操作", RequestID(r), TraceID(r))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func loginRateLimit(l *ratelimit.Limiter, ip func(*http.Request) string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, err := l.Allow(r.Context(), ip(r)+"|login", 10, time.Minute, time.Now())
		if err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "安全限流服务暂不可用", RequestID(r), TraceID(r))
			return
		}
		if !ok {
			w.Header().Set("Retry-After", "60")
			httpx.Error(w, 429, "RATE_LIMITED", "登录请求过于频繁", RequestID(r), TraceID(r))
			return
		}
		next(w, r)
	}
}

func isWrite(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}
