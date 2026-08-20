package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/sevoniva-labs/forge/internal/platform/config"
	appcrypto "github.com/sevoniva-labs/forge/internal/platform/security/crypto"
)

func TestNewCryptoProviderFailsClosedWithoutAdapter(t *testing.T) {
	_, err := newCryptoProvider(context.Background(), config.Security{CryptoKeySource: "adapter"}, nil)
	if err == nil || !strings.Contains(err.Error(), "injected KMS/HSM/password-device provider") {
		t.Fatalf("missing adapter was accepted: %v", err)
	}
}

func TestNewCryptoProviderUsesInjectedAdapter(t *testing.T) {
	security := config.Security{CryptoKeySource: "adapter"}
	provider, err := newCryptoProvider(context.Background(), security, func(_ context.Context, _ config.Security) (appcrypto.Provider, error) {
		return appcrypto.New("standard", strings.Repeat("k", 32), "hsm-v2")
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Name() != "standard" || provider.KeyVersion() != "hsm-v2" {
		t.Fatalf("unexpected injected provider: %s/%s", provider.Name(), provider.KeyVersion())
	}
}

func TestNewCryptoProviderUsesSoftwareSourceOnlyWhenSelected(t *testing.T) {
	provider, err := newCryptoProvider(context.Background(), config.Security{
		CryptoKeySource: "software", CryptoProvider: "standard", CryptoKey: strings.Repeat("k", 32), CryptoKeyVersion: "v1",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if provider.Name() != "standard" || provider.KeyVersion() != "v1" {
		t.Fatalf("unexpected software provider: %s/%s", provider.Name(), provider.KeyVersion())
	}
}
