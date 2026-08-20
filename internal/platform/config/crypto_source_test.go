package config

import (
	"strings"
	"testing"
)

func TestProductionRequiresCryptoAdapterSource(t *testing.T) {
	cfg := productionConfig()
	cfg.Security.CryptoKeySource = "software"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "crypto_key_source=adapter is required in production") {
		t.Fatalf("software crypto source was accepted in production: %v", err)
	}

	cfg.Security.CryptoKeySource = "adapter"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("adapter crypto source rejected in production: %v", err)
	}
}

func TestCryptoKeySourceRejectsUnknownValues(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=disable"
	cfg.Security.CryptoKeySource = "remote-env"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "crypto_key_source must be software|adapter") {
		t.Fatalf("unknown crypto source was accepted: %v", err)
	}
}
