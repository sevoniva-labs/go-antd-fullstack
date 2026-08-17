package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestSPARejectsAPIAndTraversalFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("index"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := SPA(root)
	for _, path := range []string{"/api/v1/missing", "/../../etc/passwd"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "index") {
			t.Fatalf("path %s returned status=%d body=%q", path, response.Code, response.Body.String())
		}
	}
}

func TestCORSDeniesUntrustedOriginAndAllowsSameOrigin(t *testing.T) {
	handler := cors(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	tests := []struct {
		origin string
		want   int
	}{{"https://evil.example", http.StatusForbidden}, {"https://forge.example", http.StatusNoContent}}
	for _, item := range tests {
		request := httptest.NewRequest(http.MethodPost, "https://forge.example/api/v1/auth/login", nil)
		request.Header.Set("Origin", item.origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != item.want {
			t.Errorf("origin %s status=%d want=%d", item.origin, response.Code, item.want)
		}
	}
}

func TestRequestIDReplacesUnsafeInput(t *testing.T) {
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) == "bad\r\nvalue" {
			t.Fatal("unsafe request ID retained")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(requestIDHeader, "bad\r\nvalue")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !validRequestID(response.Header().Get(requestIDHeader)) {
		t.Fatalf("generated request ID is invalid: %q", response.Header().Get(requestIDHeader))
	}
}

func TestMetricRouteBoundsCardinality(t *testing.T) {
	if got := metricRoute("/api/v1/admin/users/user-controlled-id"); got != "/api/v1/admin/users/:id" {
		t.Fatalf("metricRoute() = %q", got)
	}
	if got := metricRoute("/attacker/unique/path"); got != "unmatched" {
		t.Fatalf("metricRoute() = %q", got)
	}
}

func TestTracingCreatesServerSpan(t *testing.T) {
	previous := otel.GetTracerProvider()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})
	var spanContext trace.SpanContext
	handler := tracing("forge")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spanContext = trace.SpanFromContext(r.Context()).SpanContext()
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if !spanContext.IsValid() || !spanContext.IsSampled() {
		t.Fatalf("invalid server span context: %v", spanContext)
	}
}
