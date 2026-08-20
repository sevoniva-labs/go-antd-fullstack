# Federated identity sources

## Status labels

- `Built-in`: the provider-neutral port and security policy are part of the scaffold.
- `Profile`: the provider is enabled by a documented deployment profile.
- `Adapter slot`: a target-specific connector can be added without changing domain code.
- `Target-tested`: the exact issuer/directory version, TLS chain, browser flow, and failure cases passed the target contract suite.
- `Not certified`: no target evidence exists; production approval must not infer support from protocol compatibility.

## Built-in security behavior

- OIDC uses HTTPS discovery, authorization-code exchange, ID-token verification, nonce checking, and a one-time Redis/cache-backed state with a five-minute TTL.
- LDAP and Active Directory use the LDAP adapter. LDAPS or StartTLS is required by default; plaintext LDAP requires an explicit non-production override.
- External authentication never provisions a local user and never matches by email, login name, or display name.
- A local account must have an explicitly approved `(organization, provider, subject)` mapping before a federated session can be created.
- Federated sessions are marked `FEDERATED`, not `MFA`. Privileged actions still require the existing recent local MFA step-up.
- Missing provider configuration, cache state, mapping, or target authentication evidence fails closed.

## Environment configuration

OIDC is enabled only when `FORGE_OIDC_ISSUER` is set. Required values are `FORGE_OIDC_NAME`, `FORGE_OIDC_CLIENT_ID`, `FORGE_OIDC_CLIENT_SECRET` or `FORGE_OIDC_CLIENT_SECRET_FILE`, and `FORGE_OIDC_REDIRECT_URL`.

### Casdoor profile

Casdoor is used unchanged as a standard external OIDC issuer. Register a confidential client in its existing administration plane and set its redirect URI to `https://<velora-host>/api/v1/auth/federated/oidc/casdoor/callback`. The backend accepts the OIDC authorization-server browser `GET` callback (and retains the JSON `POST` callback for API clients), exchanges the authorization code server-side, and validates issuer, signature, audience, nonce, and one-time state.

This integration does not call Casdoor management APIs, use resource-owner-password login, or turn Forge into an OIDC provider. Authentication is bound only through the immutable `sub` claim and an approved local `(organization, provider, subject)` mapping. Portal resource permissions remain local policy data until a separately approved, tested Casdoor claim-to-role mapping is implemented; raw `groups`/`roles` claims must not be trusted implicitly.

LDAP/AD is enabled only when `FORGE_LDAP_URL` is set. Required values are `FORGE_LDAP_NAME`, `FORGE_LDAP_BIND_DN`, `FORGE_LDAP_BIND_PASSWORD` or `FORGE_LDAP_BIND_PASSWORD_FILE`, `FORGE_LDAP_BASE_DN`, and `FORGE_LDAP_LOGIN_ATTRIBUTE`.

`FORGE_LDAP_STARTTLS=true` enables StartTLS for an `ldap://` endpoint. `FORGE_LDAP_ALLOW_INSECURE=true` is an explicit development-only escape hatch and must not be used in a production profile.

Provider credentials are environment/file-only secrets. Do not place them in YAML, Helm values, Git, approval payloads, or audit details.

## Adapter slots

- SAML 2.0 is an `Adapter slot`: an implementation must validate the XML signature, issuer, audience, destination, recipient, `InResponseTo`, time bounds, replay, and explicit subject mapping. Unsigned assertions and implicit group-to-role mapping are forbidden.
- SCIM 2.0 is an `Adapter slot`: an implementation must use TLS, bounded pagination, ETag/`If-Match` concurrency control, explicit joiner/mover/leaver policy, deprovisioning audit, and fail-closed behavior for synchronization errors. This port never carries passwords or bearer credentials.
- No SAML/SCIM vendor is `Target-tested` until its exact product, version, TLS chain, lifecycle cases, failure behavior, and audit evidence pass the provider contract suite.

## Self-hosted development contract environment

The repository provides `deploy/compose/identity-dev.yaml` for local LDAP/OIDC contract testing. It runs an OpenLDAP-compatible directory and a Keycloak-compatible OIDC issuer as a development overlay; it does not change production identity architecture and it does not certify either product.

Import approved versions into the enterprise Harbor first, then render the overlay with immutable digest references and local-only secrets:

```bash
FORGE_LDAP_IMAGE='harbor.internal.example/approved/openldap@sha256:<digest>' \
FORGE_SSO_IMAGE='harbor.internal.example/approved/keycloak@sha256:<digest>' \
FORGE_LDAP_ADMIN_PASSWORD='local-only-admin-password' \
FORGE_LDAP_USER_PASSWORD='local-only-user-password' \
FORGE_SSO_ADMIN_PASSWORD='local-only-sso-password' \
make identity-compose-config
```

Start it only in a disposable development environment with the same variables. Configure the application against `ldap://ldap:1389` or `ldaps://ldap:1636` according to the imported image contract, and `http://sso:8080` only for local development. Production must use TLS, a managed/approved identity topology, certificate validation, MFA, backup, HA and target-specific evidence. The overlay intentionally contains no credentials or runtime data in Git.

For a real disposable runtime check, use the explicit contract target with immutable image digests and local-only environment secrets:

```bash
FORGE_LDAP_IMAGE='harbor.internal.example/approved/openldap@sha256:<digest>' \
FORGE_SSO_IMAGE='harbor.internal.example/approved/keycloak@sha256:<digest>' \
FORGE_LDAP_ADMIN_PASSWORD='local-only-admin-password' \
FORGE_LDAP_USER_PASSWORD='local-only-user-password' \
FORGE_SSO_ADMIN_PASSWORD='local-only-sso-password' \
make identity-runtime-contract
```

The contract starts the disposable overlay, verifies the LDAP test-user bind/search, the Keycloak management health endpoint, and the master-realm OIDC discovery endpoint. Set `FORGE_IDENTITY_EVIDENCE_FILE` to write a non-secret JSON evidence record containing the source commit and exact image digests. The target is `Target-tested` only for the exact image, architecture, profile, and date recorded by that evidence; it is not a production, banking, or regulatory certification.

## Evidence boundary

The scaffold provides the adapter and policy boundary. It does not certify a commercial IAM, OIDC issuer, LDAP/AD version, CA chain, browser fleet, or high-availability topology. Each target must record discovery, code exchange, nonce/state replay, TLS failure, directory timeout, disabled-account, unmapped-subject, MFA step-up, failover, and audit evidence before being labeled `Target-tested`.
