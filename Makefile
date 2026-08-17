SHELL := /bin/bash
APP ?= forge
MODULE ?= github.com/sevoniva-labs/forge
GOPROXY ?= https://goproxy.cn
GOSUMDB ?= sum.golang.google.cn
NPM_REGISTRY ?= https://registry.npmmirror.com
PNPM = corepack pnpm
GO_ENV = GOPROXY=$(GOPROXY) GOSUMDB=$(GOSUMDB)
TOOL_RUN = $(GO_ENV) go run -modfile=tools/go.mod
PROTO_TOOLS = .tools/bin/buf .tools/bin/protoc-gen-go .tools/bin/protoc-gen-go-grpc .tools/bin/protoc-gen-go-http .tools/bin/protoc-gen-openapi

.PHONY: help run worker migrate test fmt tidy web-install web-dev web-build build check contract proto-tools proto-lint proto-generate proto-breaking proto-check offline-check docker-build compose-up compose-down init ci-policy ci-go ci-web ci-deploy security-tools supply-chain-evidence release-evidence verify

help:
	@echo "Sevoniva Forge"
	@echo "  make run           Run API server"
	@echo "  make worker        Run outbox/background worker"
	@echo "  make migrate       Run one-shot database migrations"
	@echo "  make web-dev       Run frontend"
	@echo "  make test          Run Go tests"
	@echo "  make contract      Check API error-code contract"
	@echo "  make check         Format, vet, test, contract, frontend lint/build"
	@echo "  make verify        Run the complete required CI verification gate"
	@echo "  make release-evidence  Scan, sign, and verify an internal digest image"
	@echo "  make proto-check     Lint and regenerate the Buf API contracts"
	@echo "  make compose-up    Start minimal compose stack"
	@echo "  make init APP=x MODULE=example.com/x  Rename starter"

run:
	FORGE_CONFIG=configs/minimal.yaml go run ./cmd/server

worker:
	FORGE_CONFIG=configs/minimal.yaml go run ./cmd/worker

migrate:
	FORGE_CONFIG=configs/minimal.yaml go run ./cmd/migrate

test:
	go test ./...

fmt:
	gofmt -w $$(find cmd internal -name '*.go')

tidy:
	go mod tidy

web-install:
	$(PNPM) install --frozen-lockfile --registry=$(NPM_REGISTRY)

web-dev:
	$(PNPM) --filter sevoniva-forge-web dev

web-build:
	$(PNPM) --filter sevoniva-forge-web build

contract:
	python3 scripts/check-error-codes.py
	python3 scripts/check-contract-coverage.py
	python3 scripts/check-openapi-security.py

proto-tools: $(PROTO_TOOLS)

.tools/bin/buf: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/buf github.com/bufbuild/buf/cmd/buf

.tools/bin/protoc-gen-go: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go

.tools/bin/protoc-gen-go-grpc: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc

.tools/bin/protoc-gen-go-http: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/protoc-gen-go-http github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2

.tools/bin/protoc-gen-openapi: tools/go.mod tools/go.sum
	mkdir -p .tools/bin
	$(GO_ENV) go build -modfile=tools/go.mod -o .tools/bin/protoc-gen-openapi github.com/google/gnostic/cmd/protoc-gen-openapi

proto-lint: proto-tools
	.tools/bin/buf lint

proto-generate: proto-tools
	.tools/bin/buf generate --path api/proto/forge

proto-breaking: proto-tools
	.tools/bin/buf breaking --against api/buf/baseline.binpb

proto-check: proto-lint proto-breaking
	bash scripts/check-generated-proto.sh

build: web-build
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/forge ./cmd/server
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/forge-worker ./cmd/worker
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/forge-migrate ./cmd/migrate

check: fmt contract
	go vet ./...
	go test ./...
	$(PNPM) --filter sevoniva-forge-web lint
	$(PNPM) --filter sevoniva-forge-web typecheck
	$(PNPM) --filter sevoniva-forge-web test
	$(PNPM) --filter sevoniva-forge-web build

ci-policy:
	bash scripts/check-ci-policy.sh
	bash scripts/check-container-policy.sh

ci-go: ci-policy contract proto-check
	$(GO_ENV) go mod verify
	@test -z "$$(gofmt -l $$(find cmd internal -name '*.go'))" || (echo "Go files need formatting" >&2; gofmt -l $$(find cmd internal -name '*.go'); exit 1)
	$(GO_ENV) go vet ./...
	$(GO_ENV) go test ./...
	$(GO_ENV) go test -race ./...

ci-web: ci-policy web-install
	$(PNPM) --filter sevoniva-forge-web lint
	$(PNPM) --filter sevoniva-forge-web typecheck
	$(PNPM) --filter sevoniva-forge-web test
	$(PNPM) --filter sevoniva-forge-web build

ci-deploy: ci-policy
	helm lint deploy/helm/forge -f deploy/helm/forge/values-xinchuang.yaml
	helm template forge deploy/helm/forge -f deploy/helm/forge/values-xinchuang.yaml >/tmp/forge-rendered.yaml

security-tools: ci-policy
	$(TOOL_RUN) github.com/securego/gosec/v2/cmd/gosec ./...
	$(TOOL_RUN) golang.org/x/vuln/cmd/govulncheck ./...
	$(TOOL_RUN) honnef.co/go/tools/cmd/staticcheck ./...
	$(TOOL_RUN) github.com/golangci/golangci-lint/v2/cmd/golangci-lint run ./...

supply-chain-evidence: ci-policy
	bash scripts/generate-supply-chain-evidence.sh

release-evidence: supply-chain-evidence
	bash scripts/verify-image-supply-chain.sh

verify: ci-go ci-web ci-deploy security-tools supply-chain-evidence

offline-check: fmt contract
	python3 -m json.tool web/package.json >/dev/null
	bash -n scripts/init-project.sh

docker-build:
	@for value in "$$FORGE_NODE_IMAGE" "$$FORGE_GO_IMAGE" "$$FORGE_RUNTIME_IMAGE"; do \
		[[ "$$value" =~ @sha256:[0-9a-f]{64}$$ ]] || { echo "all FORGE_*_IMAGE values must be internal repository@sha256 references" >&2; exit 1; }; \
	done
	docker build -f deploy/docker/Dockerfile \
		--build-arg NODE_IMAGE="$$FORGE_NODE_IMAGE" \
		--build-arg GO_IMAGE="$$FORGE_GO_IMAGE" \
		--build-arg RUNTIME_IMAGE="$$FORGE_RUNTIME_IMAGE" \
		--build-arg NPM_REGISTRY="$(NPM_REGISTRY)" \
		--build-arg GOPROXY="$(GOPROXY)" \
		--build-arg GOSUMDB="$(GOSUMDB)" \
		-t $(APP):dev .

compose-up:
	docker compose -f deploy/compose/minimal.yaml up -d --build

compose-down:
	docker compose -f deploy/compose/minimal.yaml down -v

init:
	@test -n "$(APP)" || (echo "APP is required" && exit 1)
	@test -n "$(MODULE)" || (echo "MODULE is required" && exit 1)
	./scripts/init-project.sh "$(APP)" "$(MODULE)"
