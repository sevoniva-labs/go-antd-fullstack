package kratosapi

import (
	"context"
	"fmt"
	"io"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
)

func (s *PlatformService) DownloadDataExport(ctx context.Context, req *forgev1.DownloadDataExportRequest) (*forgev1.DownloadDataExportResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.dataPolicy == nil || strings.TrimSpace(req.GetArtifactId()) == "" {
		return nil, kratosInvalidArgument("EXPORT_ARTIFACT_REQUIRED", "artifact_id is required")
	}
	var content []byte
	var contentType, filename, digest string
	event := newAuditEvent(ctx, principal, "data_export.download", "data_export", req.GetArtifactId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		artifact, body, openErr := s.dataPolicy.OpenExport(txCtx, principal, req.GetArtifactId())
		if openErr != nil {
			return openErr
		}
		defer func() { _ = body.Close() }()
		content, openErr = io.ReadAll(body)
		if openErr != nil {
			return fmt.Errorf("read export response: %w", openErr)
		}
		contentType, digest = artifact.ContentType, artifact.SHA256
		filename = exportFilename(artifact.ID, artifact.ContentType)
		event.Details = map[string]any{"artifact_id": artifact.ID, "sha256": artifact.SHA256, "size_bytes": artifact.Size}
		return nil
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.DownloadDataExportResponse{Content: content, ContentType: contentType, Filename: filename, Sha256: digest}, nil
}

func exportFilename(id, contentType string) string {
	extension := "json"
	if strings.Contains(strings.ToLower(contentType), "csv") {
		extension = "csv"
	}
	return "data-export-" + id + "." + extension
}

func kratosInvalidArgument(code, message string) error {
	return kratoserrors.BadRequest(code, message)
}
