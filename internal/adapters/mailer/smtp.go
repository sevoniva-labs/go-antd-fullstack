package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/sevoniva-labs/forge/internal/app/notification"
)

type TLSMode string

const (
	TLSModeStartTLS TLSMode = "starttls"
	TLSModeImplicit TLSMode = "implicit"

	defaultSMTPMaxRecipients = 100
	defaultSMTPMaxMessage    = 5 * 1024 * 1024
)

var (
	ErrSMTPTLSRequired = errors.New("SMTP TLS configuration is required")
	ErrSMTPTLSWeak     = errors.New("SMTP TLS configuration must verify a TLS 1.2 or newer server")
)

type Config struct {
	Address       string
	Username      string
	Password      string
	TLSConfig     *tls.Config
	TLSMode       TLSMode
	MaxRecipients int
	MaxMessage    int
}

type SMTPClient struct {
	address       string
	host          string
	username      string
	password      string
	tlsConfig     *tls.Config
	tlsMode       TLSMode
	maxRecipients int
	maxMessage    int
}

func NewSMTPClient(config Config) (*SMTPClient, error) {
	address := strings.TrimSpace(config.Address)
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return nil, fmt.Errorf("SMTP address must be host:port: %w", err)
	}
	if config.TLSConfig == nil {
		return nil, ErrSMTPTLSRequired
	}
	if config.Username != "" && config.Password == "" || config.Username == "" && config.Password != "" {
		return nil, errors.New("SMTP username and password must be provided together")
	}
	tlsConfig := config.TLSConfig.Clone()
	if tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}
	if tlsConfig.MinVersion < tls.VersionTLS12 || tlsConfig.InsecureSkipVerify {
		return nil, ErrSMTPTLSWeak
	}
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = host
	}
	tlsMode := config.TLSMode
	if tlsMode == "" {
		tlsMode = TLSModeStartTLS
	}
	if tlsMode != TLSModeStartTLS && tlsMode != TLSModeImplicit {
		return nil, fmt.Errorf("unsupported SMTP TLS mode %q", tlsMode)
	}
	maxRecipients := config.MaxRecipients
	if maxRecipients == 0 {
		maxRecipients = defaultSMTPMaxRecipients
	}
	if maxRecipients < 1 || maxRecipients > 10000 {
		return nil, errors.New("SMTP max recipients must be between 1 and 10000")
	}
	maxMessage := config.MaxMessage
	if maxMessage == 0 {
		maxMessage = defaultSMTPMaxMessage
	}
	if maxMessage < 1024 || maxMessage > 50*1024*1024 {
		return nil, errors.New("SMTP max message must be between 1024 and 52428800 bytes")
	}
	return &SMTPClient{
		address: address, host: host, username: config.Username, password: config.Password,
		tlsConfig: tlsConfig, tlsMode: tlsMode, maxRecipients: maxRecipients, maxMessage: maxMessage,
	}, nil
}

func (c *SMTPClient) Provider() string { return "smtp-tls" }

func (c *SMTPClient) Send(ctx context.Context, message notification.Message) error {
	from, recipients, err := normalizeEnvelope(message, c.maxRecipients)
	if err != nil {
		return err
	}
	body, err := buildMessage(message, from, recipients)
	if err != nil {
		return err
	}
	if len(body) > c.maxMessage {
		return fmt.Errorf("SMTP message exceeds %d bytes", c.maxMessage)
	}
	connection, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	client, err := smtp.NewClient(connection, c.host)
	if err != nil {
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer func() { _ = client.Close() }()
	if c.tlsMode == TLSModeStartTLS {
		supported, _ := client.Extension("STARTTLS")
		if !supported {
			return errors.New("SMTP server does not advertise STARTTLS")
		}
		if err := client.StartTLS(c.tlsConfig); err != nil {
			return fmt.Errorf("SMTP STARTTLS failed: %w", err)
		}
	}
	if c.username != "" {
		supported, _ := client.Extension("AUTH")
		if !supported {
			return errors.New("SMTP server does not advertise AUTH")
		}
		if err := client.Auth(smtp.PlainAuth("", c.username, c.password, c.host)); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT TO failed: %w", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func (c *SMTPClient) dial(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if c.tlsMode == TLSModeImplicit {
		connection, err := (&tls.Dialer{NetDialer: dialer, Config: c.tlsConfig}).DialContext(ctx, "tcp", c.address)
		if err != nil {
			return nil, fmt.Errorf("dial implicit TLS SMTP: %w", err)
		}
		if deadline, ok := ctx.Deadline(); ok {
			_ = connection.SetDeadline(deadline)
		} else {
			_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
		}
		return connection, nil
	}
	connection, err := dialer.DialContext(ctx, "tcp", c.address)
	if err != nil {
		return nil, fmt.Errorf("dial SMTP: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	}
	return connection, nil
}

func normalizeEnvelope(message notification.Message, maxRecipients int) (string, []string, error) {
	from, err := normalizeMailbox(message.From)
	if err != nil {
		return "", nil, fmt.Errorf("invalid SMTP sender: %w", err)
	}
	if len(message.To) == 0 || len(message.To) > maxRecipients {
		return "", nil, fmt.Errorf("SMTP recipient count must be between 1 and %d", maxRecipients)
	}
	recipients := make([]string, 0, len(message.To))
	seen := make(map[string]struct{}, len(message.To))
	for _, raw := range message.To {
		recipient, err := normalizeMailbox(raw)
		if err != nil {
			return "", nil, fmt.Errorf("invalid SMTP recipient: %w", err)
		}
		if _, exists := seen[recipient]; exists {
			continue
		}
		seen[recipient] = struct{}{}
		recipients = append(recipients, recipient)
	}
	return from, recipients, nil
}

func normalizeMailbox(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("mailbox is empty or contains header control characters")
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address == "" || strings.ContainsAny(parsed.Address, "\r\n") {
		return "", errors.New("mailbox is not a valid address")
	}
	return parsed.Address, nil
}

func buildMessage(message notification.Message, from string, recipients []string) ([]byte, error) {
	if strings.TrimSpace(message.Subject) == "" || strings.ContainsAny(message.Subject, "\r\n") {
		return nil, errors.New("SMTP subject is required and must not contain header controls")
	}
	if message.TextBody == "" && message.HTMLBody == "" {
		return nil, errors.New("SMTP message requires a text or HTML body")
	}
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n", from, strings.Join(recipients, ", "), mime.QEncoding.Encode("UTF-8", message.Subject))
	if message.HTMLBody == "" {
		buffer.WriteString("Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
		writeCRLFBody(&buffer, message.TextBody)
		return buffer.Bytes(), nil
	}
	boundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&buffer, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)
	fmt.Fprintf(&buffer, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n", boundary)
	writeCRLFBody(&buffer, message.TextBody)
	fmt.Fprintf(&buffer, "\r\n--%s\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n", boundary)
	writeCRLFBody(&buffer, message.HTMLBody)
	fmt.Fprintf(&buffer, "\r\n--%s--\r\n", boundary)
	return buffer.Bytes(), nil
}

func writeCRLFBody(buffer *bytes.Buffer, value string) {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	_, _ = buffer.WriteString(strings.ReplaceAll(value, "\n", "\r\n"))
}

func randomBoundary() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate SMTP MIME boundary: %w", err)
	}
	return "forge-" + hex.EncodeToString(value[:]), nil
}

var _ notification.Sender = (*SMTPClient)(nil)
