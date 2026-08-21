package kratosapi

import (
	"context"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
)

func (s *PlatformService) RevokeDataExport(ctx context.Context, req *forgev1.RevokeDataExportRequest) (*forgev1.RevokeDataExportResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	artifactID := strings.TrimSpace(req.GetArtifactId())
	if artifactID == "" {
		return nil, kratosInvalidArgument("EXPORT_ARTIFACT_REQUIRED", "artifact_id is required")
	}
	reason := strings.TrimSpace(req.GetReason())
	if reason == "" || len(reason) > 512 {
		return nil, kratosInvalidArgument("EXPORT_REVOKE_REASON_REQUIRED", "a bounded revoke reason is required")
	}
	if s.dataPolicy == nil {
		return nil, kratoserrors.InternalServer("DATA_POLICY_UNAVAILABLE", "data policy service is unavailable")
	}
	event := newAuditEvent(ctx, principal, "data_export.revoke", "data_export", artifactID, map[string]any{"reason": reason})
	if err := s.audited(ctx, event, func(txCtx context.Context) error {
		return s.dataPolicy.RevokeExportState(txCtx, principal, artifactID, reason)
	}); err != nil {
		return nil, serviceError(err)
	}
	if err := s.dataPolicy.CleanupExport(ctx, principal, artifactID); err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RevokeDataExportResponse{ArtifactId: artifactID, Status: "Revoked"}, nil
}
