package identity

import (
	"context"
	"strings"
	"time"

	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
)

func (s *Service) CreateEmergencyAccess(ctx context.Context, actor domain.Principal, request domain.EmergencyAccessRequest) (domain.EmergencyAccessGrant, error) {
	if err := request.Validate(actor, time.Now().UTC()); err != nil {
		return domain.EmergencyAccessGrant{}, err
	}
	return s.repo.CreateEmergencyAccess(ctx, domain.EmergencyAccessGrant{
		OrganizationID: request.OrganizationID,
		RequesterID:    request.RequesterID,
		TargetUserID:   request.TargetUserID,
		Scope:          request.Scope,
		ApprovalID:     request.ApprovalID,
		Reason:         request.Reason,
		PrivilegeKeys:  append([]string(nil), request.PrivilegeKeys...),
		RequestedAt:    request.RequestedAt.UTC(),
		ExpiresAt:      request.ExpiresAt.UTC(),
	})
}

func (s *Service) ListEmergencyAccess(ctx context.Context, actor domain.Principal) ([]domain.EmergencyAccessGrant, error) {
	if actor.Type != "USER" || actor.UserID == "" || actor.OrganizationID == "" {
		return nil, ErrInteractiveSessionRequired
	}
	if !actor.HasRole("system_admin", "security_admin", "auditor") {
		return nil, ErrGrantCeiling
	}
	return s.repo.ListEmergencyAccess(ctx, actor.OrganizationID, 200)
}

func (s *Service) RevokeEmergencyAccess(ctx context.Context, actor domain.Principal, grantID, reason string) error {
	if err := authorizeGrantActor(actor, actor.OrganizationID); err != nil {
		return err
	}
	if !actor.HasRole("system_admin", "security_admin") {
		return ErrGrantCeiling
	}
	if strings.TrimSpace(grantID) == "" || len(strings.TrimSpace(reason)) < 8 || len(strings.TrimSpace(reason)) > 500 {
		return domain.ErrInvalidEmergencyAccess
	}
	return s.repo.RevokeEmergencyAccess(ctx, actor.OrganizationID, grantID, actor.UserID, strings.TrimSpace(reason))
}
