package authz

import (
	"testing"

	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
)

func TestPlatformRulesProtectDataExportDownload(t *testing.T) {
	rules := PlatformRules()
	permissions, ok := rules[forgev1.OperationPlatformServiceDownloadDataExport]
	if !ok || len(permissions) != 1 || permissions[0] != "system.data.export" {
		t.Fatalf("download export permission rule = %#v, want system.data.export", permissions)
	}
}
