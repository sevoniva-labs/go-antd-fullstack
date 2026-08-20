package identitysource

import (
	"testing"
	"time"
)

func TestNewLDAPProviderRequiresSecureTransportByDefault(t *testing.T) {
	_, err := NewLDAPProvider(LDAPConfig{Name: "ad", URL: "ldap://directory.example", BindDN: "cn=svc", BaseDN: "dc=example,dc=com", LoginAttribute: "sAMAccountName"})
	if err != ErrInvalidConfiguration {
		t.Fatalf("NewLDAPProvider() error = %v", err)
	}
}

func TestNewLDAPProviderAppliesSafeDefaults(t *testing.T) {
	provider, err := NewLDAPProvider(LDAPConfig{Name: "ad", URL: "ldaps://directory.example", BindDN: "cn=svc", BaseDN: "dc=example,dc=com", LoginAttribute: "sAMAccountName"})
	if err != nil {
		t.Fatalf("NewLDAPProvider() error = %v", err)
	}
	if provider.cfg.SearchTimeout != 5*time.Second || provider.cfg.DisplayAttribute != "displayName" || provider.cfg.GroupAttribute != "memberOf" {
		t.Fatalf("unexpected LDAP defaults: %#v", provider.cfg)
	}
}

func TestUniqueStringsRemovesEmptyAndDuplicates(t *testing.T) {
	got := uniqueStrings([]string{" user ", "", "user", "auditor"})
	if len(got) != 2 || got[0] != "user" || got[1] != "auditor" {
		t.Fatalf("uniqueStrings() = %#v", got)
	}
}
