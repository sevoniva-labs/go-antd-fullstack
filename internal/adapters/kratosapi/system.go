package kratosapi

import (
	"context"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
	appidentity "github.com/sevoniva-labs/forge/internal/app/identity"
	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/health"
	"google.golang.org/grpc/metadata"
)

type SystemService struct {
	forgev1.UnimplementedSystemServiceServer
	cfg       config.Config
	version   string
	identity  *appidentity.Service
	checks    []health.Check
	providers map[string]string
}

func NewSystemService(cfg config.Config, version string, identity *appidentity.Service, checks []health.Check, providers map[string]string) *SystemService {
	return &SystemService{cfg: cfg, version: version, identity: identity, checks: checks, providers: providers}
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
	token := bearerToken(ctx)
	if token == "" {
		return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	}
	if _, err := s.identity.AuthenticateAPIToken(ctx, token); err != nil {
		return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication failed")
	}
	return &forgev1.GetSystemInfoResponse{
		Service: s.cfg.App.Name, Version: s.version, Environment: s.cfg.App.Environment,
		Region: s.cfg.App.Region, Zone: s.cfg.App.Zone, Providers: s.providers,
	}, nil
}

func bearerToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, value := range md.Get("authorization") {
		parts := strings.Fields(value)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}
	return ""
}
