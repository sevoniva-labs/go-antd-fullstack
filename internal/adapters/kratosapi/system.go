package kratosapi

import (
	"context"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/forge/internal/platform/authn"
	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/health"
)

type SystemService struct {
	forgev1.UnimplementedSystemServiceServer
	cfg       config.Config
	version   string
	checks    []health.Check
	providers map[string]string
}

func NewSystemService(cfg config.Config, version string, checks []health.Check, providers map[string]string) *SystemService {
	return &SystemService{cfg: cfg, version: version, checks: checks, providers: providers}
}

func (s *SystemService) Health(context.Context, *forgev1.HealthRequest) (*forgev1.HealthResponse, error) {
	return &forgev1.HealthResponse{Status: "UP", Service: s.cfg.App.Name, Version: s.version}, nil
}

func (s *SystemService) Readiness(ctx context.Context, _ *forgev1.ReadinessRequest) (*forgev1.ReadinessResponse, error) {
	results := health.Run(ctx, s.checks)
	reply := &forgev1.ReadinessResponse{Status: "UP", Dependencies: make([]*forgev1.DependencyStatus, 0, len(results))}
	for _, result := range results {
		dependency := &forgev1.DependencyStatus{Name: result.Name, Status: result.Status}
		if result.Status != "UP" {
			reply.Status = "DOWN"
			dependency.Message = "dependency unavailable"
		}
		reply.Dependencies = append(reply.Dependencies, dependency)
	}
	return reply, nil
}

func (s *SystemService) GetSystemInfo(ctx context.Context, _ *forgev1.GetSystemInfoRequest) (*forgev1.GetSystemInfoResponse, error) {
	principal, ok := authn.Principal(ctx)
	if !ok {
		return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	}
	if !principal.HasPermission("system.status.read") {
		return nil, kratoserrors.Forbidden("PERMISSION_DENIED", "permission denied")
	}
	return &forgev1.GetSystemInfoResponse{
		Service: s.cfg.App.Name, Version: s.version, Environment: s.cfg.App.Environment,
		Region: s.cfg.App.Region, Zone: s.cfg.App.Zone, Providers: s.providers,
	}, nil
}
