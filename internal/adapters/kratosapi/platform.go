package kratosapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net"
	"strconv"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"

	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/forge/internal/app/audit"
	appidentity "github.com/sevoniva-labs/forge/internal/app/identity"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
	"github.com/sevoniva-labs/forge/internal/platform/authn"
	"github.com/sevoniva-labs/forge/internal/platform/database"
)

type PlatformService struct {
	forgev1.UnimplementedPlatformServiceServer
	identity *appidentity.Service
	audit    *audit.Writer
	db       *database.DB
}

func NewPlatformService(identity *appidentity.Service, auditWriter *audit.Writer, db *database.DB) *PlatformService {
	return &PlatformService{identity: identity, audit: auditWriter, db: db}
}

func (s *PlatformService) CreateUser(ctx context.Context, req *forgev1.CreateUserRequest) (*forgev1.CreateUserResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var created domain.User
	event := newAuditEvent(ctx, principal, "user.create", "user", "", map[string]any{"login_name": req.GetLoginName(), "roles": req.GetRoles()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.identity.CreateUser(txCtx, principal, principal.OrganizationID, req.GetLoginName(), req.GetDisplayName(), req.GetPassword(), req.GetRoles())
		if createErr == nil {
			event.ResourceID = created.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateUserResponse{User: userProto(created)}, nil
}

func (s *PlatformService) ListDepartments(ctx context.Context, _ *forgev1.ListDepartmentsRequest) (*forgev1.ListDepartmentsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.identity.ListDepartments(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListDepartmentsResponse{Departments: make([]*forgev1.Department, 0, len(items))}
	for _, item := range items {
		reply.Departments = append(reply.Departments, departmentProto(item))
	}
	return reply, nil
}

func (s *PlatformService) CreateDepartment(ctx context.Context, req *forgev1.CreateDepartmentRequest) (*forgev1.CreateDepartmentResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sortOrder, err := checkedInt(req.GetSortOrder())
	if err != nil {
		return nil, err
	}
	var created domain.Department
	event := newAuditEvent(ctx, principal, "department.create", "department", "", map[string]any{"department_key": req.GetDepartmentKey(), "parent_id": req.GetParentId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.identity.CreateDepartment(txCtx, principal, principal.OrganizationID, domain.Department{
			ParentID: req.GetParentId(), Key: req.GetDepartmentKey(), Name: req.GetName(), Status: req.GetStatus(), SortOrder: sortOrder,
		})
		if createErr == nil {
			event.ResourceID = created.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateDepartmentResponse{Department: departmentProto(created)}, nil
}

func (s *PlatformService) UpdateDepartment(ctx context.Context, req *forgev1.UpdateDepartmentRequest) (*forgev1.UpdateDepartmentResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sortOrder, err := checkedInt(req.GetSortOrder())
	if err != nil {
		return nil, err
	}
	var updated domain.Department
	event := newAuditEvent(ctx, principal, "department.update", "department", req.GetDepartmentId(), map[string]any{"parent_id": req.GetParentId(), "status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.identity.UpdateDepartment(txCtx, principal, principal.OrganizationID, req.GetDepartmentId(), domain.Department{
			ParentID: req.GetParentId(), Name: req.GetName(), Status: req.GetStatus(), SortOrder: sortOrder,
		})
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateDepartmentResponse{Department: departmentProto(updated)}, nil
}

func (s *PlatformService) UpdateOrganization(ctx context.Context, req *forgev1.UpdateOrganizationRequest) (*forgev1.UpdateOrganizationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	maxUsers, err := checkedInt(req.GetMaxUsers())
	if err != nil {
		return nil, err
	}
	maxSessions, err := checkedInt(req.GetMaxActiveSessions())
	if err != nil {
		return nil, err
	}
	var updated domain.Organization
	event := newAuditEvent(ctx, principal, "organization.update", "organization", principal.OrganizationID, map[string]any{"status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.identity.UpdateOrganization(txCtx, principal.OrganizationID, domain.Organization{
			Name: req.GetName(), Description: req.GetDescription(), Status: req.GetStatus(), MaxUsers: maxUsers, MaxSessions: maxSessions,
		})
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateOrganizationResponse{Organization: organizationProto(updated)}, nil
}

func (s *PlatformService) UpdateSecurityPolicy(ctx context.Context, req *forgev1.UpdateSecurityPolicyRequest) (*forgev1.UpdateSecurityPolicyResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	policy, err := securityPolicyDomain(req.GetPolicy())
	if err != nil {
		return nil, err
	}
	var updated domain.SecurityPolicy
	event := newAuditEvent(ctx, principal, "security.config.update", "security", "policy", nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.identity.UpdateSecurityPolicy(txCtx, principal.OrganizationID, principal.UserID, policy)
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateSecurityPolicyResponse{Policy: securityPolicyProto(updated)}, nil
}

func (s *PlatformService) UpdateRolePermissions(ctx context.Context, req *forgev1.UpdateRolePermissionsRequest) (*forgev1.UpdateRolePermissionsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "role.permissions.update", "role", req.GetRoleKey(), map[string]any{"permissions": req.GetPermissions()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.UpdateRolePermissions(txCtx, principal, principal.OrganizationID, req.GetRoleKey(), req.GetPermissions())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateRolePermissionsResponse{Role: &forgev1.Role{Key: req.GetRoleKey(), Permissions: req.GetPermissions()}}, nil
}

func (s *PlatformService) UpdateUserRoles(ctx context.Context, req *forgev1.UpdateUserRolesRequest) (*forgev1.UpdateUserRolesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user.roles.update", "user", req.GetUserId(), map[string]any{"roles": req.GetRoles()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.UpdateUserRoles(txCtx, principal, principal.OrganizationID, req.GetUserId(), req.GetRoles())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateUserRolesResponse{User: &forgev1.User{Id: req.GetUserId(), OrganizationId: principal.OrganizationID, Roles: req.GetRoles()}}, nil
}

func (s *PlatformService) UpdateUserStatus(ctx context.Context, req *forgev1.UpdateUserStatusRequest) (*forgev1.UpdateUserStatusResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user.status.update", "user", req.GetUserId(), map[string]any{"status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.SetUserStatus(txCtx, principal.OrganizationID, req.GetUserId(), req.GetStatus())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateUserStatusResponse{User: &forgev1.User{Id: req.GetUserId(), OrganizationId: principal.OrganizationID, Status: req.GetStatus()}}, nil
}

func (s *PlatformService) UnlockUser(ctx context.Context, req *forgev1.UnlockUserRequest) (*forgev1.UnlockUserResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user.unlock", "user", req.GetUserId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.UnlockUser(txCtx, principal.OrganizationID, req.GetUserId())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UnlockUserResponse{User: &forgev1.User{Id: req.GetUserId(), OrganizationId: principal.OrganizationID}}, nil
}

func (s *PlatformService) ResetUserPassword(ctx context.Context, req *forgev1.ResetUserPasswordRequest) (*forgev1.ResetUserPasswordResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user.password.reset", "user", req.GetUserId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.AdminResetPassword(txCtx, principal.OrganizationID, req.GetUserId(), req.GetPassword())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ResetUserPasswordResponse{}, nil
}

func (s *PlatformService) RevokeSession(ctx context.Context, req *forgev1.RevokeSessionRequest) (*forgev1.RevokeSessionResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "session.revoke", "session", req.GetSessionId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.RevokeSession(txCtx, principal.OrganizationID, req.GetSessionId(), principal.SessionID)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RevokeSessionResponse{}, nil
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

func (s *PlatformService) audited(ctx context.Context, event *audit.Event, operation func(context.Context) error) error {
	if s.db == nil || s.audit == nil {
		return errors.New("reliable audit is unavailable")
	}
	return s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if err := operation(txCtx); err != nil {
			return err
		}
		return s.audit.Write(txCtx, *event)
	})
}

func newAuditEvent(ctx context.Context, principal domain.Principal, action, resourceType, resourceID string, details map[string]any) *audit.Event {
	event := &audit.Event{
		OrganizationID: principal.OrganizationID, ActorID: principal.UserID, ActorName: principal.LoginName,
		Action: action, ResourceType: resourceType, ResourceID: resourceID, Details: details,
	}
	if tr, ok := transport.FromServerContext(ctx); ok {
		event.RequestID = tr.RequestHeader().Get("X-Request-ID")
	}
	if remote, ok := peer.FromContext(ctx); ok && remote.Addr != nil {
		host, _, err := net.SplitHostPort(remote.Addr.String())
		if err == nil {
			event.ClientIP = host
		}
	}
	return event
}

func checkedInt(value int64) (int, error) {
	if value < 0 || (strconv.IntSize == 32 && value > math.MaxInt32) {
		return 0, kratoserrors.BadRequest("INVALID_ARGUMENT", "numeric value is out of range")
	}
	return int(value), nil // #nosec G115 -- guarded above on 32-bit; int64 and int have equal width on 64-bit.
}

func securityPolicyDomain(policy *forgev1.SecurityPolicy) (domain.SecurityPolicy, error) {
	if policy == nil {
		return domain.SecurityPolicy{}, kratoserrors.BadRequest("INVALID_ARGUMENT", "policy is required")
	}
	minLength, err := checkedInt(policy.GetPasswordMinLength())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	history, err := checkedInt(policy.GetPasswordHistory())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	maxAge, err := checkedInt(policy.GetPasswordMaxAgeDays())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	maxFailures, err := checkedInt(policy.GetLoginMaxFailures())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	maxSessions, err := checkedInt(policy.GetMaxActiveSessions())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	return domain.SecurityPolicy{
		PasswordMinLength: minLength, PasswordRequireUpper: policy.GetPasswordRequireUpper(),
		PasswordRequireLower: policy.GetPasswordRequireLower(), PasswordRequireDigit: policy.GetPasswordRequireDigit(),
		PasswordRequireSymbol: policy.GetPasswordRequireSymbol(), PasswordHistory: history, PasswordMaxAgeDays: maxAge,
		LoginMaxFailures: maxFailures, LoginLockDurationSeconds: policy.GetLoginLockDurationSeconds(),
		SessionTTLSeconds: policy.GetSessionTtlSeconds(), MaxConcurrentSessions: maxSessions,
	}, nil
}

func serviceError(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return kratoserrors.NotFound("NOT_FOUND", "resource not found")
	case errors.Is(err, appidentity.ErrGrantCeiling), errors.Is(err, appidentity.ErrLastSystemAdmin):
		return kratoserrors.Forbidden("PERMISSION_DENIED", "operation is not permitted")
	case errors.Is(err, appidentity.ErrInteractiveSessionRequired):
		return kratoserrors.Forbidden("INTERACTIVE_SESSION_REQUIRED", "interactive session is required")
	case errors.Is(err, appidentity.ErrInvalidCredentials):
		return kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication failed")
	case errors.Is(err, appidentity.ErrInvalidRole), errors.Is(err, appidentity.ErrInvalidLoginName),
		errors.Is(err, appidentity.ErrPasswordPolicy), errors.Is(err, appidentity.ErrPasswordReused),
		errors.Is(err, appidentity.ErrInvalidSecurityPolicy), errors.Is(err, appidentity.ErrInvalidDepartment):
		return kratoserrors.BadRequest("INVALID_ARGUMENT", "request violates policy")
	default:
		return internalError(err)
	}
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

func departmentProto(department domain.Department) *forgev1.Department {
	return &forgev1.Department{
		Id: department.ID, OrganizationId: department.OrganizationID, ParentId: department.ParentID,
		DepartmentKey: department.Key, Name: department.Name, Status: department.Status,
		SortOrder: int64(department.SortOrder), CreatedAt: timestamp(department.CreatedAt), UpdatedAt: timestamp(department.UpdatedAt),
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
