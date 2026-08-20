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

LDAP/AD is enabled only when `FORGE_LDAP_URL` is set. Required values are `FORGE_LDAP_NAME`, `FORGE_LDAP_BIND_DN`, `FORGE_LDAP_BIND_PASSWORD` or `FORGE_LDAP_BIND_PASSWORD_FILE`, `FORGE_LDAP_BASE_DN`, and `FORGE_LDAP_LOGIN_ATTRIBUTE`.

`FORGE_LDAP_STARTTLS=true` enables StartTLS for an `ldap://` endpoint. `FORGE_LDAP_ALLOW_INSECURE=true` is an explicit development-only escape hatch and must not be used in a production profile.

Provider credentials are environment/file-only secrets. Do not place them in YAML, Helm values, Git, approval payloads, or audit details.

## Evidence boundary

The scaffold provides the adapter and policy boundary. It does not certify a commercial IAM, OIDC issuer, LDAP/AD version, CA chain, browser fleet, or high-availability topology. Each target must record discovery, code exchange, nonce/state replay, TLS failure, directory timeout, disabled-account, unmapped-subject, MFA step-up, failover, and audit evidence before being labeled `Target-tested`.
