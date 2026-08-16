package crypto

import "testing"

func TestProvidersRoundTrip(t *testing.T) {
	for _, name := range []string{"standard", "gm"} {
		t.Run(name, func(t *testing.T) {
			p, err := New(name, "test-key-that-is-not-for-production")
			if err != nil {
				t.Fatal(err)
			}
			ciphertext, err := p.Encrypt([]byte("secret"), []byte("aad"))
			if err != nil {
				t.Fatal(err)
			}
			plaintext, err := p.Decrypt(ciphertext, []byte("aad"))
			if err != nil {
				t.Fatal(err)
			}
			if string(plaintext) != "secret" {
				t.Fatalf("got %q", plaintext)
			}
			if _, err := p.Decrypt(ciphertext, []byte("different")); err == nil {
				t.Fatal("expected AAD verification failure")
			}
		})
	}
}
