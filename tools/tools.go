//go:build tools

package tools

import (
	_ "github.com/anchore/syft/cmd/syft"
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "github.com/zricethezav/gitleaks/v8"
	_ "github.com/securego/gosec/v2/cmd/gosec"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
