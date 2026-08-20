package storage

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func targetContract(profile ProviderProfile, capabilities ...Capability) CapabilityContract {
	results := map[Capability]CapabilityStatus{}
	for _, capability := range capabilities {
		results[capability] = CapabilityStatus{State: CapabilitySupported, Evidence: "contract test: " + string(capability)}
	}
	return CapabilityContract{
		Profile:        profile,
		Level:          EvidenceTargetTested,
		Target:         "test-target",
		EvidenceRef:    "evidence/storage-contract.json",
		EvidenceDigest: strings.Repeat("a", 64),
		TestedAt:       time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		Capabilities:   results,
	}
}

func TestCapabilityContractRequiresTargetEvidence(t *testing.T) {
	contract := defaultCapabilityContract(ProviderProfileMinIO)
	if err := contract.Validate(CapabilityMultipartRecovery); err == nil {
		t.Fatal("default profile must not enable multipart without target evidence")
	}

	contract = targetContract(ProviderProfileMinIO, CapabilityBasicObjectIO, CapabilityMultipartRecovery)
	if err := contract.Validate(CapabilityMultipartRecovery); err != nil {
		t.Fatalf("target-tested capability contract rejected: %v", err)
	}
}

func TestCapabilityContractRejectsMalformedEvidence(t *testing.T) {
	contract := targetContract(ProviderProfileCephRGW, CapabilityBasicObjectIO)
	contract.EvidenceDigest = "not-a-digest"
	if err := contract.Validate(CapabilityBasicObjectIO); err == nil {
		t.Fatal("malformed evidence digest must fail closed")
	}
}

func TestCapabilityContractRejectsWrongProfile(t *testing.T) {
	contract := targetContract(ProviderProfileAlibabaOSS, CapabilityBasicObjectIO)
	if err := contract.Validate(CapabilityObjectLock); err == nil {
		t.Fatal("unproven capability must fail closed")
	}
}

func TestLoadCapabilityContractValidatesTargetEvidence(t *testing.T) {
	path := t.TempDir() + "/cos-contract.json"
	contract := targetContract(ProviderProfileTencentCOS, CapabilityBasicObjectIO, CapabilitySSES3)
	raw, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCapabilityContract(path)
	if err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	if loaded.Profile != ProviderProfileTencentCOS || loaded.Level != EvidenceTargetTested {
		t.Fatalf("loaded contract metadata = %#v", loaded)
	}
}

func TestLoadCapabilityContractFailsClosed(t *testing.T) {
	path := t.TempDir() + "/invalid-contract.json"
	if err := os.WriteFile(path, []byte(`{"profile":"tencent-cos"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCapabilityContract(path); err == nil {
		t.Fatal("incomplete capability contract was accepted")
	}
}
