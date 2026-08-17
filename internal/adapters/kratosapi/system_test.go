package kratosapi

import (
	"context"
	"errors"
	"testing"

	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/health"
	"google.golang.org/grpc/metadata"
)

func TestSystemHealthAndReadiness(t *testing.T) {
	cfg := config.Default()
	svc := NewSystemService(cfg, "test", nil, []health.Check{
		{Name: "database", Provider: "postgres", Ping: func(context.Context) error { return nil }},
		{Name: "cache", Provider: "redis", Ping: func(context.Context) error { return errors.New("unavailable") }},
	}, map[string]string{"database": "postgres"})

	healthReply, err := svc.Health(context.Background(), &forgev1.HealthRequest{})
	if err != nil || healthReply.Status != "UP" || healthReply.Version != "test" {
		t.Fatalf("unexpected health reply: %+v, err %v", healthReply, err)
	}
	readyReply, err := svc.Readiness(context.Background(), &forgev1.ReadinessRequest{})
	if err != nil || readyReply.Status != "DOWN" || len(readyReply.Dependencies) != 2 {
		t.Fatalf("unexpected readiness reply: %+v, err %v", readyReply, err)
	}
	if readyReply.Dependencies[1].Message != "dependency unavailable" {
		t.Fatalf("dependency error was not safely normalized: %+v", readyReply.Dependencies[1])
	}
}

func TestSystemInfoRequiresBearerAuthentication(t *testing.T) {
	svc := NewSystemService(config.Default(), "test", nil, nil, nil)
	if _, err := svc.GetSystemInfo(context.Background(), &forgev1.GetSystemInfoRequest{}); err == nil {
		t.Fatal("system info accepted an unauthenticated request")
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer machine-token"))
	if token := bearerToken(ctx); token != "machine-token" {
		t.Fatalf("bearer token = %q", token)
	}
}
