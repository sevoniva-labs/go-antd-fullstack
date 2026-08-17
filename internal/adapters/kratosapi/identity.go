package kratosapi

import (
	"context"
	"time"

	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/forge/internal/app/audit"
	appidentity "github.com/sevoniva-labs/forge/internal/app/identity"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
	"github.com/sevoniva-labs/forge/internal/platform/database"
)

type IdentityService struct {
	forgev1.UnimplementedIdentityServiceServer
	identity *appidentity.Service
	audit    *audit.Writer
	db       *database.DB
}

func NewIdentityService(identity *appidentity.Service, auditWriter *audit.Writer, db *database.DB) *IdentityService {
	return &IdentityService{identity: identity, audit: auditWriter, db: db}
}

func (s *IdentityService) GetCurrentUser(ctx context.Context, _ *forgev1.GetCurrentUserRequest) (*forgev1.GetCurrentUserResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	return &forgev1.GetCurrentUserResponse{User: &forgev1.User{
		Id: principal.UserID, OrganizationId: principal.OrganizationID, LoginName: principal.LoginName,
		DisplayName: principal.DisplayName, MustChangePassword: principal.MustChangePassword,
		PasswordChangedAt: timestamp(principal.PasswordChangedAt), Roles: principal.Roles, Permissions: principal.Permissions,
	}}, nil
}

func (s *IdentityService) ChangePassword(ctx context.Context, req *forgev1.ChangePasswordRequest) (*forgev1.ChangePasswordResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "auth.password.change", "user", principal.UserID, nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.ChangePassword(txCtx, principal, req.GetCurrentPassword(), req.GetNewPassword())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ChangePasswordResponse{}, nil
}

func (s *IdentityService) ListApiTokens(ctx context.Context, _ *forgev1.ListApiTokensRequest) (*forgev1.ListApiTokensResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	tokens, err := s.identity.ListAPITokens(ctx, principal)
	if err != nil {
		return nil, serviceError(err)
	}
	reply := &forgev1.ListApiTokensResponse{Tokens: make([]*forgev1.ApiToken, 0, len(tokens))}
	for _, token := range tokens {
		reply.Tokens = append(reply.Tokens, apiTokenProto(token))
	}
	return reply, nil
}

func (s *IdentityService) CreateApiToken(ctx context.Context, req *forgev1.CreateApiTokenRequest) (*forgev1.CreateApiTokenResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	days := req.GetExpiresDays()
	if days == 0 {
		days = 90
	}
	if days < 1 || days > 365 {
		return nil, serviceError(appidentity.ErrInvalidSecurityPolicy)
	}
	var token domain.APIToken
	var secret string
	event := newAuditEvent(ctx, principal, "security.api_token.create", "api_token", "", map[string]any{"name": req.GetName(), "scopes": req.GetScopes()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		token, secret, createErr = s.identity.CreateAPIToken(txCtx, principal, req.GetName(), req.GetScopes(), time.Duration(days)*24*time.Hour)
		if createErr == nil {
			event.ResourceID = token.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateApiTokenResponse{Token: apiTokenProto(token), Secret: secret}, nil
}

func (s *IdentityService) RevokeApiToken(ctx context.Context, req *forgev1.RevokeApiTokenRequest) (*forgev1.RevokeApiTokenResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "security.api_token.revoke", "api_token", req.GetTokenId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.RevokeAPIToken(txCtx, principal, req.GetTokenId())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RevokeApiTokenResponse{}, nil
}

func (s *IdentityService) audited(ctx context.Context, event *audit.Event, operation func(context.Context) error) error {
	platform := PlatformService{audit: s.audit, db: s.db}
	return platform.audited(ctx, event, operation)
}

func apiTokenProto(token domain.APIToken) *forgev1.ApiToken {
	return &forgev1.ApiToken{
		Id: token.ID, Name: token.Name, Prefix: token.Prefix, Scopes: token.Scopes,
		CreatedAt: timestamp(token.CreatedAt), ExpiresAt: optionalTimestamp(token.ExpiresAt), LastUsedAt: optionalTimestamp(token.LastUsedAt),
	}
}
