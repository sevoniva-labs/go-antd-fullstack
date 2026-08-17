package kratosapi

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/sevoniva-labs/forge/internal/app/audit"
	domain "github.com/sevoniva-labs/forge/internal/domain/identity"
)

func TestPlatformProtoMappings(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	locked := now.Add(time.Minute)
	user := userProto(domain.User{
		ID: "user-1", OrganizationID: "org-1", LoginName: "alice", Status: "ACTIVE",
		LockedUntil: &locked, CreatedAt: now, Roles: []string{"auditor"},
	})
	if user.Id != "user-1" || user.OrganizationId != "org-1" || user.LockedUntil.AsTime() != locked {
		t.Fatalf("unexpected user mapping: %+v", user)
	}
	policy := securityPolicyProto(domain.SecurityPolicy{PasswordMinLength: 14, SessionTTLSeconds: 3600, MaxConcurrentSessions: 2})
	if policy.PasswordMinLength != 14 || policy.SessionTtlSeconds != 3600 || policy.MaxActiveSessions != 2 {
		t.Fatalf("unexpected policy mapping: %+v", policy)
	}
	event := auditEventProto(audit.Event{ID: "event-1", OccurredAt: now, Details: map[string]any{"safe": true}})
	if event.Id != "event-1" || event.DetailsJson != `{"safe":true}` {
		t.Fatalf("unexpected audit mapping: %+v", event)
	}
}

func TestPlatformNumericMappingsRejectInvalidValues(t *testing.T) {
	if _, err := checkedInt(-1); err == nil {
		t.Fatal("negative quota was accepted")
	}
	if strconv.IntSize == 32 {
		if _, err := checkedInt(int64(math.MaxInt32) + 1); err == nil {
			t.Fatal("overflowing 32-bit quota was accepted")
		}
	}
	if _, err := securityPolicyDomain(nil); err == nil {
		t.Fatal("missing security policy was accepted")
	}
}
