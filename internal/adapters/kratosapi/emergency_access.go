package kratosapi

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
	appapproval "github.com/sevoniva-labs/forge/internal/app/approval"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
)

func (s *PlatformService) ListEmergencyAccess(ctx context.Context, _ *forgev1.ListEmergencyAccessRequest) (*forgev1.ListEmergencyAccessResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	grants, err := s.identity.ListEmergencyAccess(ctx, principal)
	if err != nil {
		return nil, serviceError(err)
	}
	response := &forgev1.ListEmergencyAccessResponse{Grants: make([]*forgev1.EmergencyAccessGrant, 0, len(grants))}
	for _, grant := range grants {
		response.Grants = append(response.Grants, emergencyAccessProto(grant))
	}
	return response, nil
}

func (s *PlatformService) CreateEmergencyAccess(ctx context.Context, req *forgev1.CreateEmergencyAccessRequest) (*forgev1.CreateEmergencyAccessResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetExpiresAt() == nil {
		return nil, serviceError(domain.ErrInvalidEmergencyAccess)
	}
	privilegeKeys := normalizedPrivilegeKeys(req.GetPrivilegeKeys())
	requestedAt := time.Now().UTC()
	expiresAt := req.GetExpiresAt().AsTime().UTC()
	targetUserID := strings.TrimSpace(req.GetTargetUserId())
	scope := strings.TrimSpace(req.GetScope())
	approvalID := strings.TrimSpace(req.GetApprovalId())
	reason := strings.TrimSpace(req.GetReason())
	payloadBytes, err := json.Marshal(map[string]any{
		"expires_at": expiresAt.Format(time.RFC3339Nano), "privilege_keys": privilegeKeys,
		"reason": reason, "scope": scope, "target_user_id": targetUserID,
	})
	if err != nil {
		return nil, internalError(err)
	}
	request := domain.EmergencyAccessRequest{
		OrganizationID: principal.OrganizationID, RequesterID: principal.UserID, TargetUserID: targetUserID,
		Scope: scope, ApprovalID: approvalID, Reason: reason, PrivilegeKeys: privilegeKeys,
		RequestedAt: requestedAt, ExpiresAt: expiresAt,
	}
	resourceID := targetUserID
	if resourceID == "" {
		resourceID = scope
	}
	var grant domain.EmergencyAccessGrant
	event := newAuditEvent(ctx, principal, "emergency_access.create", "emergency_access", resourceID, map[string]any{
		"approval_id": approvalID, "expires_at": expiresAt, "privilege_count": len(privilegeKeys), "scope": scope,
	})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, approvalID, appapproval.ExecutionInput{
			RequestType: "EMERGENCY_ACCESS", Action: "emergency_access.create", Resource: "emergency_access", ResourceID: resourceID, PayloadJSON: string(payloadBytes),
		}); executionErr != nil {
			return executionErr
		}
		var createErr error
		grant, createErr = s.identity.CreateEmergencyAccess(txCtx, principal, request)
		if createErr == nil {
			event.ResourceID = grant.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateEmergencyAccessResponse{Grant: emergencyAccessProto(grant)}, nil
}

func (s *PlatformService) RevokeEmergencyAccess(ctx context.Context, req *forgev1.RevokeEmergencyAccessRequest) (*forgev1.RevokeEmergencyAccessResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "emergency_access.revoke", "emergency_access", req.GetGrantId(), map[string]any{"reason": req.GetReason()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.RevokeEmergencyAccess(txCtx, principal, req.GetGrantId(), req.GetReason())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RevokeEmergencyAccessResponse{}, nil
}

func normalizedPrivilegeKeys(values []string) []string {
	keys := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func emergencyAccessProto(grant domain.EmergencyAccessGrant) *forgev1.EmergencyAccessGrant {
	status := "SCHEDULED"
	now := time.Now().UTC()
	if grant.RevokedAt != nil {
		status = "REVOKED"
	} else if !now.Before(grant.ExpiresAt) {
		status = "EXPIRED"
	} else if !now.Before(grant.RequestedAt) {
		status = "ACTIVE"
	}
	return &forgev1.EmergencyAccessGrant{
		Id: grant.ID, OrganizationId: grant.OrganizationID, RequesterId: grant.RequesterID, TargetUserId: grant.TargetUserID,
		Scope: grant.Scope, ApprovalId: grant.ApprovalID, Reason: grant.Reason, PrivilegeKeys: append([]string(nil), grant.PrivilegeKeys...),
		RequestedAt: timestamp(grant.RequestedAt), ExpiresAt: timestamp(grant.ExpiresAt), Status: status,
		RevokedAt: optionalTimestamp(grant.RevokedAt), RevokedBy: grant.RevokedBy, RevokeReason: grant.RevokeReason, CreatedAt: timestamp(grant.CreatedAt),
	}
}
