package identitysource

import "context"

// SAMLProvider is an adapter slot for a target-specific SAML 2.0 service
// provider. Implementations must validate XML signatures, issuer, audience,
// destination, recipient, InResponseTo, and assertion time bounds before
// returning a federated identity. The scaffold intentionally ships no SAML
// implementation or claim-to-role mapping.
type SAMLProvider interface {
	Name() string
	Begin(ctx context.Context, relayState string) (SAMLRedirect, error)
	Complete(ctx context.Context, response []byte, relayState string) (FederatedIdentity, error)
}

type SAMLRedirect struct {
	URL string
}

// SCIMProvider is an adapter slot for a target-specific SCIM 2.0 lifecycle
// connector. Implementations must use TLS, bounded pagination, ETag/If-Match
// concurrency checks, explicit deprovisioning policy, and auditable failures.
// Passwords and bearer credentials are deliberately absent from this port.
type SCIMProvider interface {
	Name() string
	ListUsers(ctx context.Context, cursor string, pageSize int) (SCIMUserPage, error)
	DisableUser(ctx context.Context, subject string) error
}

type SCIMUserPage struct {
	Users      []SCIMUser
	NextCursor string
}

type SCIMUser struct {
	Subject     string
	LoginName   string
	DisplayName string
	Email       string
	Active      bool
	Groups      []string
}
