# Validation

## Offline checks performed on this generated package

The artifact-generation environment had no external Go/npm registry access, so validation was deliberately split into **offline structural checks** and **networked dependency-aware checks**.

Offline checks passed:

- 44 Go source files parsed/formatted by `gofmt`
- fixed six-digit API error-code contract: all literal handler symbols are registered
- 21 YAML files parsed: OpenAPI, configs, Compose, observability, Helm values/Chart and GitHub workflows
- 5 JSON frontend/tooling files parsed
- 18 TypeScript/TSX files parsed with zero syntax diagnostics
- all frontend relative imports resolved to local source files
- Bash syntax check for `scripts/init-project.sh`
- project initializer smoke test (`Acme Portal`, `example.com/acme/portal`)
- Helm template control-block balance check
- API and Worker Kubernetes Deployment selectors verified as non-overlapping (`component=api|worker`)
- no `InsecureSkipVerify` / `skip_verify` production escape hatch
- no private-key files shipped
- bootstrap password and application crypto key remain empty in source-controlled examples

## Required checks in a networked environment

Run once after cloning/generating a real project:

```bash
go mod tidy
go vet ./...
go test ./...
cd web
npm install
npm run lint
npm run typecheck
npm run test
npm run build
cd ..

python3 scripts/check-error-codes.py
docker build -f deploy/docker/Dockerfile .
helm lint deploy/helm/forge
helm template forge deploy/helm/forge -f deploy/helm/forge/values.yaml >/tmp/forge-rendered.yaml
```

CI in this repo also tracks these checks in `.github/workflows/ci.yml` for each push/PR.

Then commit `go.sum` and the chosen frontend lockfile so dependency resolution becomes reproducible. The supplied CI/security/release workflows add OpenAPI lint, multi-architecture image builds, dependency vulnerability checks, SAST-style Go checks, secret scanning, Trivy, CycloneDX SBOM, BuildKit provenance/SBOM attestations and a Cosign signing baseline.

## Why full compilation is not claimed here

The local runtime could parse Go source but could not download the Go 1.26 toolchain/modules or npm dependencies. Therefore this package does **not** claim dependency-aware `go test`, frontend bundling, Docker build or Helm render success in the generation environment. Those are explicitly left as first-networked-run gates rather than being reported as passed without evidence.
