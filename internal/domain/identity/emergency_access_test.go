package identity

import (
	"errors"
	"testing"
	"time"
)

func validEmergencyAccessRequest(now time.Time) (Principal, EmergencyAccessRequest) {
	verified := now.Add(-2 * time.Minute)
	return Principal{Type: "USER", UserID: "operator-1", OrganizationID: "org-1", SessionID: "session-1", Roles: []string{"security_admin"}, MFAVerifiedAt: &verified}, EmergencyAccessRequest{
		OrganizationID: "org-1", RequesterID: "operator-1", TargetUserID: "user-2", Scope: "production-config",
		ApprovalID: "approval-1", Reason: "restore an approved production service", PrivilegeKeys: []string{"system.config.read"},
		RequestedAt: now.Add(-time.Minute), ExpiresAt: now.Add(20 * time.Minute),
	}
}

func TestEmergencyAccessRequiresRecentMFAAndBoundApproval(t *testing.T) {
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	actor, request := validEmergencyAccessRequest(now)
	if err := request.Validate(actor, now); err != nil {
		t.Fatalf("valid emergency access rejected: %v", err)
	}

	request.ApprovalID = ""
	if !errors.Is(request.Validate(actor, now), ErrInvalidEmergencyAccess) {
		t.Fatal("missing approval was accepted")
	}
	request.ApprovalID = "approval-1"
	old := now.Add(-EmergencyAccessMFAMaxAge - time.Second)
	actor.MFAVerifiedAt = &old
	if !errors.Is(request.Validate(actor, now), ErrEmergencyAccessDenied) {
		t.Fatal("stale MFA was accepted")
	}
}

func TestEmergencyAccessRejectsTokensWildcardsAndSelfAccess(t *testing.T) {
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	actor, request := validEmergencyAccessRequest(now)

	tests := map[string]func(*Principal, *EmergencyAccessRequest){
		"token":       func(a *Principal, _ *EmergencyAccessRequest) { a.Type = "TOKEN" },
		"self access": func(_ *Principal, r *EmergencyAccessRequest) { r.TargetUserID = "operator-1" },
		"wildcard":    func(_ *Principal, r *EmergencyAccessRequest) { r.PrivilegeKeys = []string{"*"} },
		"excessive duration": func(_ *Principal, r *EmergencyAccessRequest) {
			r.ExpiresAt = r.RequestedAt.Add(EmergencyAccessMaxAge + time.Minute)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			copyActor, copyRequest := actor, request
			mutate(&copyActor, &copyRequest)
			if err := copyRequest.Validate(copyActor, now); err == nil {
				t.Fatal("unsafe emergency access was accepted")
			}
		})
	}
}
