# Prometheus runtime contract

The scaffold provides a disposable Prometheus contract overlay. It checks readiness, the HTTP query API, and a self-scraped up metric using an immutable image and the existing middleware evidence format.

~~~bash
PROMETHEUS_IMAGE='harbor.internal.example/approved/prometheus@sha256:<digest>' \
PROMETHEUS_VERSION='v2.55.1' \
PROMETHEUS_PLATFORM='linux/arm64' \
FORGE_MIDDLEWARE_EVIDENCE_FILE='.evidence/prometheus-runtime.json' \
make prometheus-runtime-contract
make middleware-evidence-check
~~~

This is a local runtime contract at level Experimental. It does not certify an institution Prometheus cluster, long-term retention, HA, remote write, Alertmanager, TLS/mTLS, capacity, or regulatory monitoring controls.
