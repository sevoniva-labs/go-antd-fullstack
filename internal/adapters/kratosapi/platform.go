package kratosapi

import (
	"context"
	"encoding/json"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/forge/internal/app/audit"
	appidentity "github.com/sevoniva-labs/forge/internal/app/identity"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
	"github.com/sevoniva-labs/forge/internal/platform/authn"
)

type PlatformService struct {
	forgev1.UnimplementedPlatformServiceServer
	identity *appidentity.Service
	audit    *audit.Writer
}

func NewPlatformService(identity *appidentity.Service, auditWriter *audit.Writer) *PlatformService {
	return &PlatformService{identity: identity, audit: auditWriter}
}

func (s *PlatformService) ListUsers(ctx context.Context, _ *forgev1.ListUsersRequest) (*forgev1.ListUsersResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.identity.ListUsers(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListUsersResponse{Users: make([]*forgev1.User, 0, len(users))}
	for _, user := range users {
		reply.Users = append(reply.Users, userProto(user))
	}
	return reply, nil
}

func (s *PlatformService) GetOrganization(ctx context.Context, _ *forgev1.GetOrganizationRequest) (*forgev1.GetOrganizationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	organization, err := s.identity.Organization(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.GetOrganizationResponse{Organization: organizationProto(organization)}, nil
}

func (s *PlatformService) GetSecurityPolicy(ctx context.Context, _ *forgev1.GetSecurityPolicyRequest) (*forgev1.GetSecurityPolicyResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	policy, err := s.identity.SecurityPolicy(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.GetSecurityPolicyResponse{Policy: securityPolicyProto(policy)}, nil
}

func (s *PlatformService) ListRoles(ctx context.Context, _ *forgev1.ListRolesRequest) (*forgev1.ListRolesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := s.identity.ListRoles(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListRolesResponse{Roles: make([]*forgev1.Role, 0, len(roles))}
	for _, role := range roles {
		reply.Roles = append(reply.Roles, roleProto(role))
	}
	return reply, nil
}

func (s *PlatformService) ListPermissions(ctx context.Context, _ *forgev1.ListPermissionsRequest) (*forgev1.ListPermissionsResponse, error) {
	permissions, err := s.identity.ListPermissions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListPermissionsResponse{Permissions: make([]*forgev1.Permission, 0, len(permissions))}
	for _, permission := range permissions {
		reply.Permissions = append(reply.Permissions, permissionProto(permission))
	}
	return reply, nil
}

func (s *PlatformService) ListSessions(ctx context.Context, _ *forgev1.ListSessionsRequest) (*forgev1.ListSessionsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := s.identity.ListSessions(ctx, principal.OrganizationID, principal.SessionID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListSessionsResponse{Sessions: make([]*forgev1.Session, 0, len(sessions))}
	for _, session := range sessions {
		reply.Sessions = append(reply.Sessions, sessionProto(session))
	}
	return reply, nil
}

func (s *PlatformService) ListAuditLogs(ctx context.Context, req *forgev1.ListAuditLogsRequest) (*forgev1.ListAuditLogsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	limit := int(req.GetLimit())
	events, err := s.audit.List(ctx, principal.OrganizationID, limit)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListAuditLogsResponse{Events: make([]*forgev1.AuditEvent, 0, len(events))}
	for _, event := range events {
		reply.Events = append(reply.Events, auditEventProto(event))
	}
	return reply, nil
}

func requiredPrincipal(ctx context.Context) (domain.Principal, error) {
	principal, ok := authn.Principal(ctx)
	if !ok || principal.OrganizationID == "" {
		return domain.Principal{}, kratoserrors.Unauthorized("UNAUTHENTICATED", "authenticated organization is required")
	}
	return principal, nil
}

func internalError(error) error {
	return kratoserrors.InternalServer("INTERNAL", "operation failed")
}

func timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func optionalTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamp(*value)
}

func userProto(user domain.User) *forgev1.User {
	return &forgev1.User{
		Id: user.ID, OrganizationId: user.OrganizationID, LoginName: user.LoginName,
		DisplayName: user.DisplayName, Status: user.Status, MustChangePassword: user.MustChangePassword,
		LockedUntil: optionalTimestamp(user.LockedUntil), PasswordChangedAt: timestamp(user.PasswordChangedAt),
		CreatedAt: timestamp(user.CreatedAt), Roles: user.Roles, Permissions: user.Permissions,
	}
}

func organizationProto(organization domain.Organization) *forgev1.Organization {
	return &forgev1.Organization{
		Id: organization.ID, OrganizationKey: organization.Key, Name: organization.Name,
		Description: organization.Description, Status: organization.Status, MaxUsers: int64(organization.MaxUsers),
		MaxActiveSessions: int64(organization.MaxSessions), CreatedAt: timestamp(organization.CreatedAt),
		UpdatedAt: timestamp(organization.UpdatedAt),
	}
}

func securityPolicyProto(policy domain.SecurityPolicy) *forgev1.SecurityPolicy {
	return &forgev1.SecurityPolicy{
		PasswordMinLength: int64(policy.PasswordMinLength), PasswordRequireUpper: policy.PasswordRequireUpper,
		PasswordRequireLower: policy.PasswordRequireLower, PasswordRequireDigit: policy.PasswordRequireDigit,
		PasswordRequireSymbol: policy.PasswordRequireSymbol, PasswordHistory: int64(policy.PasswordHistory),
		PasswordMaxAgeDays: int64(policy.PasswordMaxAgeDays), LoginMaxFailures: int64(policy.LoginMaxFailures),
		LoginLockDurationSeconds: policy.LoginLockDurationSeconds, SessionTtlSeconds: policy.SessionTTLSeconds,
		MaxActiveSessions: int64(policy.MaxConcurrentSessions),
	}
}

func permissionProto(permission domain.Permission) *forgev1.Permission {
	return &forgev1.Permission{Key: permission.Key, Name: permission.Name}
}

func roleProto(role domain.Role) *forgev1.Role {
	permissions := make([]string, 0, len(role.Permissions))
	for _, permission := range role.Permissions {
		permissions = append(permissions, permission.Key)
	}
	return &forgev1.Role{Key: role.Key, Name: role.Name, Permissions: permissions}
}

func sessionProto(session domain.Session) *forgev1.Session {
	return &forgev1.Session{
		Id: session.ID, UserId: session.UserID, LoginName: session.LoginName,
		ClientIp: session.ClientIP, UserAgent: session.UserAgent, CreatedAt: timestamp(session.CreatedAt),
		ExpiresAt: timestamp(session.ExpiresAt), LastSeenAt: timestamp(session.LastSeenAt), Current: session.Current,
	}
}

func auditEventProto(event audit.Event) *forgev1.AuditEvent {
	details, _ := json.Marshal(event.Details)
	return &forgev1.AuditEvent{
		Id: event.ID, OccurredAt: timestamp(event.OccurredAt), RequestId: event.RequestID,
		OrganizationId: event.OrganizationID, ActorId: event.ActorID, ActorName: event.ActorName,
		Action: event.Action, ResourceType: event.ResourceType, ResourceId: event.ResourceID,
		Result: event.Result, ClientIp: event.ClientIP, DetailsJson: string(details),
	}
}
