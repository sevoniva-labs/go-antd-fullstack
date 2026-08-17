package httpapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sevoniva-labs/forge/internal/app/audit"
)

func encodeAuditCSV(items []audit.Event) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	header := []string{
		"id", "occurred_at", "request_id", "organization_id", "actor_id", "actor_name",
		"action", "resource_type", "resource_id", "result", "client_ip", "details",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write CSV header: %w", err)
	}
	for _, item := range items {
		details, err := json.Marshal(item.Details)
		if err != nil {
			return nil, fmt.Errorf("encode audit details: %w", err)
		}
		record := sanitizeCSVRecord([]string{
			item.ID, item.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
			item.RequestID, item.OrganizationID, item.ActorID, item.ActorName, item.Action,
			item.ResourceType, item.ResourceID, item.Result, item.ClientIP, string(details),
		})
		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("write CSV record: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV: %w", err)
	}
	return buf.Bytes(), nil
}

func sanitizeCSVRecord(record []string) []string {
	clean := make([]string, len(record))
	for index, value := range record {
		clean[index] = sanitizeCSVCell(value)
	}
	return clean
}

func sanitizeCSVCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}
