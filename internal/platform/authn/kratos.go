package authn

import (
	"context"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
)

type Authenticator interface {
	AuthenticateAPIToken(context.Context, string) (domain.Principal, error)
}

type principalContextKey struct{}

func Server(authenticator Authenticator) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "transport context is required")
			}
			token := bearerToken(tr.RequestHeader().Get("Authorization"))
			if token == "" {
				return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "Bearer token is required")
			}
			principal, err := authenticator.AuthenticateAPIToken(ctx, token)
			if err != nil {
				return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication failed")
			}
			return next(WithPrincipal(ctx, principal), req)
		}
	}
}

func WithPrincipal(ctx context.Context, principal domain.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func Principal(ctx context.Context) (domain.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(domain.Principal)
	return principal, ok
}

func bearerToken(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
