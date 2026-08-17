package kratosapi

import "testing"

func TestUserRoleChangePayloadIsDeterministic(t *testing.T) {
	roles, payload, err := userRoleChangePayload([]string{" auditor ", "user", "auditor", ""})
	if err != nil {
		t.Fatalf("userRoleChangePayload() error = %v", err)
	}
	if len(roles) != 2 || roles[0] != "auditor" || roles[1] != "user" {
		t.Fatalf("userRoleChangePayload() roles = %#v", roles)
	}
	if payload != `{"roles":["auditor","user"]}` {
		t.Fatalf("userRoleChangePayload() payload = %s", payload)
	}
}
