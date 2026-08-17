package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
	"github.com/sevoniva-labs/forge/internal/platform/ratelimit"
)

func TestAllowedBeforePasswordChange(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{name: "current identity", method: http.MethodGet, path: "/api/v1/me", want: true},
		{name: "change password", method: http.MethodPatch, path: "/api/v1/auth/password", want: true},
		{name: "logout", method: http.MethodPost, path: "/api/v1/auth/logout", want: true},
		{name: "read users", method: http.MethodGet, path: "/api/v1/admin/users", want: false},
		{name: "read audit", method: http.MethodGet, path: "/api/v1/admin/audit-logs", want: false},
		{name: "create token", method: http.MethodPost, path: "/api/v1/api-tokens", want: false},
		{name: "wrong password method", method: http.MethodGet, path: "/api/v1/auth/password", want: false},
		{name: "password path suffix", method: http.MethodPatch, path: "/api/v1/auth/password/extra", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allowedBeforePasswordChange(tt.method, tt.path); got != tt.want {
				t.Fatalf("allowedBeforePasswordChange(%q, %q) = %v, want %v", tt.method, tt.path, got, tt.want)
			}
		})
	}
}

func TestPasswordChangeRateLimit(t *testing.T) {
	limiter := ratelimit.New(nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := passwordChangeRateLimit(limiter, func(*http.Request) string { return "192.0.2.10" })(next)
	principal := domain.Principal{Type: "USER", UserID: "u1", OrganizationID: "org1", SessionID: "session1"}

	for attempt := 1; attempt <= 6; attempt++ {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/password", nil)
		req = req.WithContext(withPrincipal(req.Context(), principal))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if attempt <= 5 && res.Code != http.StatusNoContent {
			t.Fatalf("attempt %d status = %d, want %d", attempt, res.Code, http.StatusNoContent)
		}
		if attempt == 6 {
			if res.Code != http.StatusTooManyRequests {
				t.Fatalf("attempt %d status = %d, want %d", attempt, res.Code, http.StatusTooManyRequests)
			}
			if got := res.Header().Get("Retry-After"); got != "900" {
				t.Fatalf("Retry-After = %q, want 900", got)
			}
		}
	}
}
