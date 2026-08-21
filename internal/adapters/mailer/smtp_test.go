package mailer

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/sevoniva-labs/forge/internal/app/notification"
)

func TestNewSMTPClientRequiresVerifiedTLS(t *testing.T) {
	if _, err := NewSMTPClient(Config{Address: "localhost:2525"}); !errors.Is(err, ErrSMTPTLSRequired) {
		t.Fatalf("missing TLS config error = %v", err)
	}
	if _, err := NewSMTPClient(Config{Address: "localhost:2525", TLSConfig: &tls.Config{InsecureSkipVerify: true}}); !errors.Is(err, ErrSMTPTLSWeak) {
		t.Fatalf("insecure TLS config error = %v", err)
	}
	if _, err := NewSMTPClient(Config{Address: "localhost:2525", TLSConfig: &tls.Config{}, Username: "user"}); err == nil {
		t.Fatal("partial SMTP credentials were accepted")
	}
}

func TestBuildMessageRejectsHeaderInjection(t *testing.T) {
	if _, err := buildMessage(notification.Message{Subject: "x\r\nBcc: attacker@example.com", TextBody: "body"}, "sender@example.com", []string{"receiver@example.com"}); err == nil {
		t.Fatal("subject header injection was accepted")
	}
	if _, err := normalizeMailbox("Attacker\r\nBcc: attacker@example.com"); err == nil {
		t.Fatal("mailbox header injection was accepted")
	}
}

func TestSMTPClientSendsSTARTTLSMessage(t *testing.T) {
	certificate, roots := testCertificate(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	received := make(chan string, 1)
	errorsCh := make(chan error, 1)
	go runTestSMTPServer(listener, certificate, received, errorsCh)

	client, err := NewSMTPClient(Config{
		Address: listener.Addr().String(), TLSConfig: &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = client.Send(context.Background(), notification.Message{
		From: "sender@example.com", To: []string{"receiver@example.com"}, Subject: "测试通知", TextBody: "plain body", HTMLBody: "<b>html body</b>",
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errorsCh:
		t.Fatal(err)
	case body := <-received:
		if !strings.Contains(body, "Content-Type: multipart/alternative") || !strings.Contains(body, "plain body") || !strings.Contains(body, "<b>html body</b>") {
			t.Fatalf("SMTP body missing expected parts: %s", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
}

func runTestSMTPServer(listener net.Listener, certificate tls.Certificate, received chan<- string, errorsCh chan<- error) {
	connection, err := listener.Accept()
	if err != nil {
		errorsCh <- err
		return
	}
	defer func() { _ = connection.Close() }()
	reader := bufio.NewReader(connection)
	if _, err := fmt.Fprint(connection, "220 localhost ESMTP\r\n"); err != nil {
		errorsCh <- err
		return
	}
	if err := expectSMTPCommand(reader, connection, "EHLO", "250-localhost\r\n250-STARTTLS\r\n250 AUTH PLAIN\r\n"); err != nil {
		errorsCh <- err
		return
	}
	if err := expectSMTPCommand(reader, connection, "STARTTLS", "220 Ready to start TLS\r\n"); err != nil {
		errorsCh <- err
		return
	}
	tlsConnection := tls.Server(connection, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
	if err := tlsConnection.Handshake(); err != nil {
		errorsCh <- err
		return
	}
	reader = bufio.NewReader(tlsConnection)
	if err := expectSMTPCommand(reader, tlsConnection, "EHLO", "250-localhost\r\n250 AUTH PLAIN\r\n"); err != nil {
		errorsCh <- err
		return
	}
	if err := expectSMTPCommand(reader, tlsConnection, "MAIL FROM:", "250 sender ok\r\n"); err != nil {
		errorsCh <- err
		return
	}
	if err := expectSMTPCommand(reader, tlsConnection, "RCPT TO:", "250 recipient ok\r\n"); err != nil {
		errorsCh <- err
		return
	}
	if err := expectSMTPCommand(reader, tlsConnection, "DATA", "354 continue\r\n"); err != nil {
		errorsCh <- err
		return
	}
	var body strings.Builder
	for {
		line, readErr := readSMTPLine(reader)
		if readErr != nil {
			errorsCh <- readErr
			return
		}
		if line == "." {
			break
		}
		body.WriteString(line)
		body.WriteString("\n")
	}
	if _, err := fmt.Fprint(tlsConnection, "250 queued\r\n"); err != nil {
		errorsCh <- err
		return
	}
	if err := expectSMTPCommand(reader, tlsConnection, "QUIT", "221 bye\r\n"); err != nil {
		errorsCh <- err
		return
	}
	received <- body.String()
}

func expectSMTPCommand(reader *bufio.Reader, connection net.Conn, prefix, response string) error {
	line, err := readSMTPLine(reader)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, prefix) {
		return fmt.Errorf("expected SMTP command %q, got %q", prefix, line)
	}
	_, err = io.WriteString(connection, response)
	return err
}

func readSMTPLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

func testCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
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
