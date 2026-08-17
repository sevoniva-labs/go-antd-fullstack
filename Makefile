SHELL := /bin/bash
APP ?= forge
MODULE ?= github.com/sevoniva-labs/forge
GOPROXY ?= https://goproxy.cn
GOSUMDB ?= sum.golang.google.cn
NPM_REGISTRY ?= https://registry.npmmirror.com
GO_ENV = GOPROXY=$(GOPROXY) GOSUMDB=$(GOSUMDB)
TOOL_RUN = $(GO_ENV) go run -modfile=tools/go.mod

.PHONY: help run worker migrate test fmt tidy web-install web-dev web-build build check contract offline-check docker-build compose-up compose-down init ci-policy ci-go ci-web ci-deploy security-tools

help:
	@echo "Sevoniva Forge"
	@echo "  make run           Run API server"
	@echo "  make worker        Run outbox/background worker"
	@echo "  make migrate       Run one-shot database migrations"
	@echo "  make web-dev       Run frontend"
	@echo "  make test          Run Go tests"
	@echo "  make contract      Check API error-code contract"
	@echo "  make check         Format, vet, test, contract, frontend lint/build"
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
	cd web && npm install --registry=$(NPM_REGISTRY)

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

contract:
	python3 scripts/check-error-codes.py

build: web-build
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/forge ./cmd/server
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/forge-worker ./cmd/worker
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$${VERSION:-dev}" -o bin/forge-migrate ./cmd/migrate

check: fmt contract
	go vet ./...
	go test ./...
	cd web && npm run lint
	cd web && npm run typecheck
	cd web && npm run test
	cd web && npm run build

ci-policy:
	bash scripts/check-ci-policy.sh

ci-go: ci-policy contract
	$(GO_ENV) go mod verify
	@test -z "$$(gofmt -l $$(find cmd internal -name '*.go'))" || (echo "Go files need formatting" >&2; gofmt -l $$(find cmd internal -name '*.go'); exit 1)
	$(GO_ENV) go vet ./...
	$(GO_ENV) go test ./...
	$(GO_ENV) go test -race ./...

ci-web: ci-policy web-install
	cd web && npm run lint
	cd web && npm run typecheck
	cd web && npm run test
	cd web && npm run build

ci-deploy: ci-policy
	helm lint deploy/helm/forge -f deploy/helm/forge/values-xinchuang.yaml
	helm template forge deploy/helm/forge -f deploy/helm/forge/values-xinchuang.yaml >/tmp/forge-rendered.yaml

security-tools: ci-policy
	$(TOOL_RUN) github.com/securego/gosec/v2/cmd/gosec ./...
	$(TOOL_RUN) golang.org/x/vuln/cmd/govulncheck ./...
	$(TOOL_RUN) honnef.co/go/tools/cmd/staticcheck ./...
	$(TOOL_RUN) github.com/golangci/golangci-lint/v2/cmd/golangci-lint run ./...

offline-check: fmt contract
	python3 -m json.tool web/package.json >/dev/null
	bash -n scripts/init-project.sh

docker-build:
	docker build -f deploy/docker/Dockerfile -t $(APP):dev .

compose-up:
	docker compose -f deploy/compose/minimal.yaml up -d --build

compose-down:
	docker compose -f deploy/compose/minimal.yaml down -v

init:
	@test -n "$(APP)" || (echo "APP is required" && exit 1)
	@test -n "$(MODULE)" || (echo "MODULE is required" && exit 1)
	./scripts/init-project.sh "$(APP)" "$(MODULE)"
