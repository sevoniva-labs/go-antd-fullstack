package config

import (
	"strings"
	"testing"
)

func TestAllS3ProfilesUseProductionTLSGuards(t *testing.T) {
	for _, provider := range []string{"s3", "aws-s3", "amazon-s3", "s3-compatible", "minio", "ceph-rgw", "alibaba-oss", "tencent-cos", "huawei-obs"} {
		t.Run(provider, func(t *testing.T) {
			cfg := productionConfig()
			cfg.Storage.Provider = provider
			cfg.Storage.Bucket = "documents"
			cfg.Storage.Endpoint = "https://storage.internal"
			cfg.Storage.TLS = true
			if err := cfg.Validate(); err != nil {
				t.Fatalf("secure S3 profile rejected: %v", err)
			}

			cfg.Storage.TLS = false
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "storage.tls must be enabled for s3") {
				t.Fatalf("insecure S3 profile was accepted: %v", err)
			}
		})
	}
}

func TestUnknownStorageProfileIsRejected(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=disable"
	cfg.Storage.Provider = "unknown-object-store"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "storage.provider must be local|s3") {
		t.Fatalf("unknown storage profile was accepted: %v", err)
	}
}
