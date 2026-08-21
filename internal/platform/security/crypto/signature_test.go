package crypto

import (
	"bytes"
	"context"
	"testing"
)

func TestSM2SoftwareSignerSignsAndVerifies(t *testing.T) {
	privateKey := bytes.Repeat([]byte{1}, 32)
	signer, err := NewSM2SoftwareSigner(privateKey, []byte("forge-signature-test"))
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("approval payload")
	signature, err := signer.Sign(context.Background(), message)
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.Verify(context.Background(), message, signature); err != nil {
		t.Fatal(err)
	}
	if err := signer.Verify(context.Background(), []byte("tampered payload"), signature); err == nil {
		t.Fatal("expected tampered message to fail verification")
	}
}

func TestSM2SoftwareSignerRequiresExplicitInputs(t *testing.T) {
	if _, err := NewSM2SoftwareSigner([]byte("short"), []byte("uid")); err == nil {
		t.Fatal("expected private-key length validation")
	}
	if _, err := NewSM2SoftwareSigner(bytes.Repeat([]byte{1}, 32), nil); err == nil {
		t.Fatal("expected explicit user-id validation")
	}

	signer, err := NewSM2SoftwareSigner(bytes.Repeat([]byte{1}, 32), []byte("uid"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := signer.Sign(ctx, []byte("message")); err == nil {
		t.Fatal("expected canceled context to fail signing")
	}
}
