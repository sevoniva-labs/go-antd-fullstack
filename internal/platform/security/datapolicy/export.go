package datapolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatCSV  ExportFormat = "csv"

	DefaultExportMaxRows = 5000
	MaximumExportMaxRows = 100000
)

var (
	ErrNoExportFields       = errors.New("at least one data field is required for export")
	ErrInvalidExportFormat  = errors.New("export format must be json or csv")
	ErrExportRowLimit       = errors.New("data export row limit exceeded")
	ErrExportFieldMissing   = errors.New("data export row is missing a requested field")
	ErrDuplicateExportField = errors.New("data export contains duplicate fields")
)

// ExportFormat controls the representation of an already-authorized export.
// Storage, download expiry, and revocation stay outside this package.
type ExportFormat string

// ExportOptions is the immutable request context that must be bound to the
// approval record by the application layer before RenderExport is called.
type ExportOptions struct {
	Fields     []string
	Purpose    string
	ApprovalID string
	Watermark  string
	Format     ExportFormat
	MaxRows    int
}

// ExportRow is intentionally a string map. Business query code should convert
// its typed result into this explicit export projection instead of allowing a
// generic serializer to infer sensitive fields.
type ExportRow map[string]string

// ExportArtifact is ready for a governed storage adapter. SHA256 is computed
// over Content and can be recorded in an audit event or object metadata.
type ExportArtifact struct {
	Content     []byte
	ContentType string
	Format      ExportFormat
	Fields      []string
	RowCount    int
	Watermark   string
	SHA256      string
}

// RenderExport applies the registered field policies and renders only the
// requested projection. It fails closed when approval metadata, a field
// registration, a row field, or a row limit is invalid.
func (c *Catalog) RenderExport(options ExportOptions, rows []ExportRow) (ExportArtifact, error) {
	fields, err := normalizeExportFields(options.Fields)
	if err != nil {
		return ExportArtifact{}, err
	}
	format := options.Format
	if format == "" {
		format = ExportFormatJSON
	}
	if format != ExportFormatJSON && format != ExportFormatCSV {
		return ExportArtifact{}, ErrInvalidExportFormat
	}
	maxRows := options.MaxRows
	if maxRows == 0 {
		maxRows = DefaultExportMaxRows
	}
	if maxRows < 1 || maxRows > MaximumExportMaxRows {
		return ExportArtifact{}, fmt.Errorf("%w: must be between 1 and %d", ErrExportRowLimit, MaximumExportMaxRows)
	}
	if len(rows) > maxRows {
		return ExportArtifact{}, fmt.Errorf("%w: got %d, maximum %d", ErrExportRowLimit, len(rows), maxRows)
	}
	if err := c.AuthorizeExport(fields, ExportRequest{
		ApprovalID: options.ApprovalID,
		Purpose:    options.Purpose,
		Watermark:  options.Watermark,
	}); err != nil {
		return ExportArtifact{}, err
	}

	values := make([][]string, 0, len(rows))
	for rowIndex, row := range rows {
		valueRow := make([]string, 0, len(fields))
		for _, field := range fields {
			value, ok := row[field]
			if !ok {
				return ExportArtifact{}, fmt.Errorf("%w: field %q at row %d", ErrExportFieldMissing, field, rowIndex)
			}
			masked, maskErr := c.MaskValue(field, value)
			if maskErr != nil {
				return ExportArtifact{}, maskErr
			}
			valueRow = append(valueRow, masked)
		}
		values = append(values, valueRow)
	}

	content, contentType, err := renderExportContent(format, fields, values, options.Watermark)
	if err != nil {
		return ExportArtifact{}, err
	}
	digest := sha256.Sum256(content)
	return ExportArtifact{
		Content:     content,
		ContentType: contentType,
		Format:      format,
		Fields:      append([]string(nil), fields...),
		RowCount:    len(values),
		Watermark:   options.Watermark,
		SHA256:      fmt.Sprintf("%x", digest[:]),
	}, nil
}

func normalizeExportFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return nil, ErrNoExportFields
	}
	normalized := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			return nil, ErrNoExportFields
		}
		if _, exists := seen[field]; exists {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateExportField, field)
		}
		seen[field] = struct{}{}
		normalized = append(normalized, field)
	}
	return normalized, nil
}

type exportDocument struct {
	Fields    []string   `json:"fields"`
	Rows      [][]string `json:"rows"`
	Watermark string     `json:"watermark,omitempty"`
}

func renderExportContent(format ExportFormat, fields []string, rows [][]string, watermark string) ([]byte, string, error) {
	if format == ExportFormatJSON {
		content, err := json.Marshal(exportDocument{
			Fields:    fields,
			Rows:      rows,
			Watermark: watermark,
		})
		return content, "application/json; charset=utf-8", err
	}

	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	header := append(append([]string(nil), fields...), "watermark")
	for index := range header {
		header[index] = sanitizeCSVCell(header[index])
	}
	if err := writer.Write(header); err != nil {
		return nil, "", err
	}
	for _, row := range rows {
		record := append(append([]string(nil), row...), watermark)
		for index := range record {
			record[index] = sanitizeCSVCell(record[index])
		}
		if err := writer.Write(record); err != nil {
			return nil, "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), "text/csv; charset=utf-8", nil
}

func sanitizeCSVCell(value string) string {
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\ufeff'
	})
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

// ExportExpiry is a small helper for storage adapters that need to bind an
// artifact to a short-lived download policy without placing time semantics in
// the renderer itself.
func ExportExpiry(now time.Time, lifetime time.Duration) (time.Time, error) {
	if now.IsZero() || lifetime <= 0 {
		return time.Time{}, errors.New("export expiry requires a valid time and positive lifetime")
	}
	return now.UTC().Add(lifetime), nil
}
