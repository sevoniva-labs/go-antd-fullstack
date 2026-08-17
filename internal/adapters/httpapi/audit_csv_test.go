package httpapi

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/sevoniva-labs/forge/internal/app/audit"
)

func TestSanitizeCSVCell(t *testing.T) {
	tests := map[string]string{
		"=HYPERLINK(\"https://example.invalid\")": "'=HYPERLINK(\"https://example.invalid\")",
		"+cmd":       "'+cmd",
		"-1+2":       "'-1+2",
		"@SUM(A1)":   "'@SUM(A1)",
		"  =cmd":     "'  =cmd",
		"\t@cmd":     "'\t@cmd",
		"safe-value": "safe-value",
		"":           "",
	}
	for input, want := range tests {
		if got := sanitizeCSVCell(input); got != want {
			t.Errorf("sanitizeCSVCell(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEncodeAuditCSVSanitizesEveryField(t *testing.T) {
	data, err := encodeAuditCSV([]audit.Event{{
		ID: "=id", OccurredAt: time.Unix(0, 0).UTC(), RequestID: "+request",
		OrganizationID: "-org", ActorID: "@actor", ActorName: "=name", Action: "+action",
		ResourceType: "-type", ResourceID: "@resource", Result: "=result", ClientIP: "+ip",
		Details: map[string]any{"safe": "value"},
	}})
	if err != nil {
		t.Fatalf("encodeAuditCSV() error = %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatalf("parse encoded CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	for index, value := range records[1][:11] {
		if index == 1 {
			continue
		}
		if !strings.HasPrefix(value, "'") {
			t.Errorf("field %d was not neutralized: %q", index, value)
		}
	}
}
