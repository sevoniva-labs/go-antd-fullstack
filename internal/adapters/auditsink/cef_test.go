package auditsink

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sevoniva-labs/forge/internal/app/audit"
)

func TestEncodeCEFUsesSafeMetadataAndEscaping(t *testing.T) {
	payload, err := EncodeCEF(audit.Event{
		Action: "user|update", ResourceType: "user", ResourceID: "id=1", Result: "FAILURE",
		Details: map[string]any{"secret": "must not be sent"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(payload, "CEF:0|Forge|Forge|1|user.user\\|update|user\\|update|8|") {
		t.Fatalf("unexpected CEF header: %s", payload)
	}
	if !strings.Contains(payload, "resource_id=id\\=1") || strings.Contains(payload, "must not be sent") {
		t.Fatalf("CEF escaping or details minimization failed: %s", payload)
	}
}

func TestNewCEFClientRequiresVerifiedTLS(t *testing.T) {
	if _, err := NewCEFClient("localhost:6514", nil); !errors.Is(err, ErrSIEMTLSRequired) {
		t.Fatalf("nil TLS config error = %v", err)
	}
	if _, err := NewCEFClient("localhost:6514", &tls.Config{InsecureSkipVerify: true}); !errors.Is(err, ErrSIEMTLSWeak) {
		t.Fatalf("insecure TLS config error = %v", err)
	}
}

func TestCEFClientPublishesOctetCountedTLSFrame(t *testing.T) {
	certificate, roots := testTLSCertificate(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	received := make(chan []byte, 1)
	errorsCh := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			errorsCh <- acceptErr
			return
		}
		defer func() { _ = connection.Close() }()
		data, readErr := io.ReadAll(connection)
		if readErr != nil {
			errorsCh <- readErr
			return
		}
		received <- data
	}()

	client, err := NewCEFClient(listener.Addr().String(), &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Publish(context.Background(), audit.Event{Action: "audit.export", Result: "SUCCESS", EventHash: "hash-1"}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errorsCh:
		t.Fatal(err)
	case frame := <-received:
		separator := bytes.IndexByte(frame, ' ')
		if separator < 1 {
			t.Fatalf("CEF frame has no octet-count separator: %q", frame)
		}
		declared, err := strconv.Atoi(string(frame[:separator]))
		if err != nil || declared != len(frame[separator+1:]) {
			t.Fatalf("invalid octet count %q for %d bytes", frame[:separator], len(frame[separator+1:]))
		}
		if !strings.Contains(string(frame[separator+1:]), "event_hash=hash-1") {
			t.Fatalf("CEF event missing event hash: %q", frame)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for CEF frame")
	}
}

func testTLSCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return certificate, roots
}
