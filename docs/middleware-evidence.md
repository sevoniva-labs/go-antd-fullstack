# Middleware runtime evidence

The RocketMQ, Kafka, Nacos, and OTel runtime contracts can emit the common
`forge-middleware-runtime-evidence` record by setting `FORGE_MIDDLEWARE_EVIDENCE_FILE`.
The legacy provider-specific evidence variables remain supported for local
compatibility, but they are not a production or Xinchuang certification.

## Required metadata

When standard evidence is enabled, the caller must provide all target metadata
explicitly. The scripts do not infer a vendor version or silently claim support:

```sh
export FORGE_MIDDLEWARE_EVIDENCE_FILE=.evidence/rocketmq-standard.json
export FORGE_MIDDLEWARE_EVIDENCE_LEVEL='Experimental'
export FORGE_MIDDLEWARE_PRODUCT='RocketMQ'
export FORGE_MIDDLEWARE_VERSION='5'
export FORGE_MIDDLEWARE_ARCHITECTURE='arm64'
export FORGE_MIDDLEWARE_OS='linux'
export FORGE_MIDDLEWARE_RUNTIME='docker'
export FORGE_MIDDLEWARE_DRIVER='rocketmq-go-v5'
make rocketmq-runtime-contract
make middleware-evidence-check
```

The runtime scripts always require immutable lowercase image digests. Standard
evidence records contain no credentials and are limited to the checks actually
executed by the contract. `Experimental` is the appropriate level for the
standalone development overlays currently in this repository.

`Target-tested` additionally requires an evidence root, safe relative evidence
references, SHA-256 digests, and every declared check to be `passed`:

```sh
FORGE_MIDDLEWARE_EVIDENCE_FILE=/path/to/evidence.json \
FORGE_MIDDLEWARE_EVIDENCE_ROOT=/path/to/evidence-root \
make middleware-evidence-check-certified
```

This does not certify TLS, ACL, HA, persistence recovery, Xinchuang hardware,
regulatory controls, or a production topology unless those controls are
explicitly executed and included in target evidence.
