package httpapi

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/sevoniva-labs/forge/internal/domain/identity"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	principalKey contextKey = "principal"
)

func RequestID(r *http.Request) string { v, _ := r.Context().Value(requestIDKey).(string); return v }
func Principal(r *http.Request) *identity.Principal {
	v, _ := r.Context().Value(principalKey).(*identity.Principal)
	return v
}
func withPrincipal(ctx context.Context, p identity.Principal) context.Context {
	return context.WithValue(ctx, principalKey, &p)
}

func TraceID(r *http.Request) string {
	sc := trace.SpanContextFromContext(r.Context())
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}
