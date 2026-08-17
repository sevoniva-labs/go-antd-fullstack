# Forge APISIX integration

This optional chart creates APISIX Ingress Controller resources for the Forge HTTP and gRPC endpoints. It does not install APISIX itself. Mirror the approved APISIX and Ingress Controller charts and images into the organization's domestic Helm registry and Harbor before installation; no public-registry fallback is permitted.

## Prerequisites

- APISIX Ingress Controller 2.1.x with the `apisix.apache.org/v2` CRDs installed.
- The Forge chart installed in the same namespace and its Service name supplied as `service.name`.
- Existing Kubernetes TLS Secrets for browser ingress, gRPC ingress, and the APISIX client certificate used toward Forge.
- The Forge server configured with TLS and client-certificate verification for APISIX-to-Forge traffic.

Render and review before applying:

```sh
helm template forge-apisix deploy/helm/forge-apisix \
  --namespace forge \
  --set enabled=true \
  --set hosts.web=forge.bank.example.cn \
  --set hosts.grpc=grpc.forge.bank.example.cn
```

The chart deliberately sets upstream retries to zero. Retrying non-idempotent financial operations at the gateway can duplicate side effects; add application idempotency before changing this policy. The default rate limit is local to each APISIX instance. Replace it with an approved centralized Redis-backed policy when a cluster-wide quota is required, and inject Redis credentials through the organization's secret manager rather than values files.

## TLS boundary

Ingress TLS terminates at APISIX. APISIX then uses `https` and `grpcs` and presents the client certificate referenced by `upstream.clientCertificateSecretName`; Forge must verify that certificate. APISIX currently does not verify the legality of upstream server certificates, so this is not symmetric end-to-end identity verification. Keep Forge ingress restricted to APISIX Pods with NetworkPolicy, use dedicated short-lived certificates, and record this residual risk in the deployment security assessment.

References:

- https://apisix.apache.org/docs/ingress-controller/getting-started/configure-routes/
- https://apisix.apache.org/docs/ingress-controller/next/reference/apisix-ingress-controller/api-reference/
- https://apisix.apache.org/docs/apisix/FAQ/
