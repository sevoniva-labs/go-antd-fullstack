package discovery

import (
	"testing"

	"github.com/sevoniva-labs/forge/internal/platform/config"
)

func TestBuildEndpointsSeparatesProtocols(t *testing.T) {
	endpoints := buildEndpoints(config.Discovery{
		ServiceName: "account-http", GRPCServiceName: "account-grpc",
		AdvertisePort: 8080, AdvertiseGRPCPort: 9090,
	}, "account", map[string]string{"version": "v1"})
	if len(endpoints) != 2 {
		t.Fatalf("endpoint count = %d, want 2", len(endpoints))
	}
	if endpoints[0].service != "account-http" || endpoints[0].metadata["protocol"] != "http" || endpoints[0].metadata["port"] != "8080" {
		t.Fatalf("unexpected HTTP endpoint: %#v", endpoints[0])
	}
	if endpoints[1].service != "account-grpc" || endpoints[1].metadata["protocol"] != "grpc" || endpoints[1].metadata["port"] != "9090" {
		t.Fatalf("unexpected gRPC endpoint: %#v", endpoints[1])
	}
	endpoints[0].metadata["protocol"] = "changed"
	if endpoints[1].metadata["protocol"] != "grpc" {
		t.Fatal("endpoint metadata maps must not alias")
	}
}

func TestBuildEndpointsUsesProtocolSpecificDefaults(t *testing.T) {
	endpoints := buildEndpoints(config.Discovery{AdvertisePort: 8080, AdvertiseGRPCPort: 9090}, "forge", nil)
	if endpoints[0].service != "forge-http" || endpoints[1].service != "forge-grpc" {
		t.Fatalf("unexpected default services: %#v", endpoints)
	}
}
