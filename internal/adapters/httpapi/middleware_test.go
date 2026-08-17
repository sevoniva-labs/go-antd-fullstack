package httpapi

import (
	"net/http"
	"testing"
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
