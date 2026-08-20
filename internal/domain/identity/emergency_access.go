package identity

import (
	"errors"
	"strings"
	"time"
)

const (
	EmergencyAccessMFAMaxAge = 10 * time.Minute
	EmergencyAccessMaxAge    = 60 * time.Minute
)

var (
	ErrInvalidEmergencyAccess = errors.New("invalid emergency access request")
	ErrEmergencyAccessDenied  = errors.New("emergency access policy denied")
)

// EmergencyAccessRequest is the policy-bound input for a break-glass session.
// Persistence and approval adapters must retain the exact request digest.
type EmergencyAccessRequest struct {
	OrganizationID string
	RequesterID    string
	TargetUserID   string
	Scope          string
	ApprovalID     string
	Reason         string
	PrivilegeKeys  []string
	RequestedAt    time.Time
	ExpiresAt      time.Time
}

func (r EmergencyAccessRequest) Validate(actor Principal, now time.Time) error {
	now = now.UTC()
	if now.IsZero() || actor.Type != "USER" || strings.TrimSpace(actor.UserID) == "" || strings.TrimSpace(actor.OrganizationID) == "" || strings.TrimSpace(actor.SessionID) == "" {
		return ErrEmergencyAccessDenied
	}
	if !actor.HasRole("system_admin", "security_admin") || actor.MFAVerifiedAt == nil {
		return ErrEmergencyAccessDenied
	}
	verifiedAt := actor.MFAVerifiedAt.UTC()
	if verifiedAt.After(now) || now.Sub(verifiedAt) > EmergencyAccessMFAMaxAge {
		return ErrEmergencyAccessDenied
	}
	if strings.TrimSpace(r.OrganizationID) != actor.OrganizationID || strings.TrimSpace(r.RequesterID) != actor.UserID {
		return ErrEmergencyAccessDenied
	}
	if strings.TrimSpace(r.TargetUserID) == actor.UserID || (strings.TrimSpace(r.TargetUserID) == "" && strings.TrimSpace(r.Scope) == "") {
		return ErrInvalidEmergencyAccess
	}
	if strings.TrimSpace(r.ApprovalID) == "" || len(strings.TrimSpace(r.Reason)) < 8 || len(strings.TrimSpace(r.Reason)) > 500 {
		return ErrInvalidEmergencyAccess
	}
	if r.RequestedAt.IsZero() || r.ExpiresAt.IsZero() || r.ExpiresAt.Before(r.RequestedAt) || !r.ExpiresAt.After(now) || r.ExpiresAt.Sub(r.RequestedAt) > EmergencyAccessMaxAge {
		return ErrInvalidEmergencyAccess
	}
	if len(r.PrivilegeKeys) == 0 || len(r.PrivilegeKeys) > 20 {
		return ErrInvalidEmergencyAccess
	}
	for _, key := range r.PrivilegeKeys {
		key = strings.TrimSpace(key)
		if key == "" || key == "*" || key == "system_admin" || strings.ContainsAny(key, "\r\n") {
			return ErrInvalidEmergencyAccess
		}
	}
	return nil
}
