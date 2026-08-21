package datapolicy

import (
	"errors"
	"strings"
	"testing"
)

func TestCatalogRequiresControlsForPersonalInformation(t *testing.T) {
	if _, err := NewCatalog([]FieldPolicy{{
		Key: "customer.mobile", Classification: ClassificationPersonalInformation,
		Owner: "retail", Purpose: "customer service", Residency: "cn", RetentionDays: 365, Mask: MaskNone,
	}}); err == nil {
		t.Fatal("personal information without masking was accepted")
	}
	catalog, err := NewCatalog([]FieldPolicy{{
		Key: "customer.mobile", Classification: ClassificationPersonalInformation,
		Owner: "retail", Purpose: "customer service", Residency: "cn", RetentionDays: 365, Mask: MaskMobile,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := catalog.MaskValue("customer.mobile", "13812345678"); err != nil || got != "138****5678" {
		t.Fatalf("MaskValue() = %q, %v", got, err)
	}
	if err := catalog.AuthorizeExport([]string{"customer.mobile"}, ExportRequest{Purpose: "case review"}); err == nil {
		t.Fatal("sensitive export without approval was accepted")
	}
	if err := catalog.AuthorizeExport([]string{"customer.mobile"}, ExportRequest{Purpose: "case review", ApprovalID: "approval-1", Watermark: "case-1"}); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogRejectsUnknownAndUnregisteredFields(t *testing.T) {
	if _, err := NewCatalog([]FieldPolicy{{
		Key: "x", Classification: "unknown", Owner: "team", Purpose: "test", Residency: "cn", RetentionDays: 1, Mask: MaskNone,
	}}); err == nil {
		t.Fatal("unknown classification was accepted")
	}
	catalog, err := NewCatalog([]FieldPolicy{{
		Key: "x", Classification: ClassificationPublic, Owner: "team", Purpose: "test", Residency: "cn", RetentionDays: 1, Mask: MaskNone,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.MaskValue("missing", "value"); err == nil {
		t.Fatal("unregistered field was accepted")
	}
}

func TestCatalogRendersMaskedWatermarkedCSV(t *testing.T) {
	catalog, err := NewCatalog([]FieldPolicy{
		{Key: "customer.mobile", Classification: ClassificationPersonalInformation, Owner: "retail", Purpose: "service", Residency: "cn", RetentionDays: 365, Mask: MaskMobile},
		{Key: "customer.note", Classification: ClassificationPublic, Owner: "retail", Purpose: "service", Residency: "cn", RetentionDays: 365, Mask: MaskNone},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := catalog.RenderExport(ExportOptions{
		Fields: []string{"customer.mobile", "customer.note"}, Purpose: "case review", ApprovalID: "approval-1", Watermark: "case-1", Format: ExportFormatCSV,
	}, []ExportRow{{"customer.mobile": "13812345678", "customer.note": "=not-a-formula", "ignored": "must not export"}})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ContentType != "text/csv; charset=utf-8" || artifact.RowCount != 1 || artifact.SHA256 == "" {
		t.Fatalf("unexpected artifact metadata: %+v", artifact)
	}
	content := string(artifact.Content)
	if !strings.Contains(content, "138****5678") || !strings.Contains(content, "' =not-a-formula") && !strings.Contains(content, "'=not-a-formula") || !strings.Contains(content, "case-1") {
		t.Fatalf("CSV did not enforce masking, formula protection, and watermark: %q", content)
	}
	if strings.Contains(content, "must not export") {
		t.Fatal("unrequested field was exported")
	}
}

func TestCatalogRenderExportFailsClosed(t *testing.T) {
	catalog, err := NewCatalog([]FieldPolicy{{
		Key: "customer.mobile", Classification: ClassificationPersonalInformation, Owner: "retail", Purpose: "service", Residency: "cn", RetentionDays: 365, Mask: MaskMobile,
	}})
	if err != nil {
		t.Fatal(err)
	}
	base := ExportOptions{Fields: []string{"customer.mobile"}, Purpose: "case review", ApprovalID: "approval-1", Watermark: "case-1"}
	if _, err := catalog.RenderExport(base, []ExportRow{{}}); !errors.Is(err, ErrExportFieldMissing) {
		t.Fatalf("missing row field error = %v", err)
	}
	base.Fields = []string{"unknown"}
	if _, err := catalog.RenderExport(base, []ExportRow{{"unknown": "secret"}}); err == nil {
		t.Fatal("unregistered field was exported")
	}
	base.Fields = []string{"customer.mobile"}
	base.ApprovalID = ""
	if _, err := catalog.RenderExport(base, []ExportRow{{"customer.mobile": "13812345678"}}); err == nil {
		t.Fatal("export without approval was accepted")
	}
	base.ApprovalID = "approval-1"
	base.MaxRows = 1
	if _, err := catalog.RenderExport(base, []ExportRow{{"customer.mobile": "13812345678"}, {"customer.mobile": "13912345678"}}); !errors.Is(err, ErrExportRowLimit) {
		t.Fatalf("row limit error = %v", err)
	}
}

func TestCatalogRendersJSONProjection(t *testing.T) {
	catalog, err := NewCatalog([]FieldPolicy{{
		Key: "customer.name", Classification: ClassificationPersonalInformation, Owner: "retail", Purpose: "service", Residency: "cn", RetentionDays: 365, Mask: MaskName,
	}})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := catalog.RenderExport(ExportOptions{Fields: []string{"customer.name"}, Purpose: "case review", ApprovalID: "approval-1", Watermark: "case-1"}, []ExportRow{{"customer.name": "张三"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(artifact.Content), `"watermark":"case-1"`) || !strings.Contains(string(artifact.Content), `"张*"`) {
		t.Fatalf("JSON export missing governed projection: %s", artifact.Content)
	}
}
