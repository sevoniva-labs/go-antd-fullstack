package auditsink

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/sevoniva-labs/forge/internal/app/audit"
)

const maxCEFEventBytes = 64 * 1024

var (
	ErrSIEMTLSRequired = errors.New("siem TLS configuration is required")
	ErrSIEMTLSWeak     = errors.New("siem TLS configuration must verify a TLS 1.2 or newer server")
)

// CEFClient sends provider-neutral Common Event Format payloads over a TLS
// syslog endpoint. RFC 6587 octet-counting prevents payload data from being
// mistaken for a message delimiter when details contain newlines.
type CEFClient struct {
	address   string
	tlsConfig *tls.Config
}

func NewCEFClient(address string, config *tls.Config) (*CEFClient, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("siem address is required")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return nil, fmt.Errorf("siem address must be host:port: %w", err)
	}
	if config == nil {
		return nil, ErrSIEMTLSRequired
	}
	cloned := config.Clone()
	if cloned.MinVersion == 0 {
		cloned.MinVersion = tls.VersionTLS12
	}
	if cloned.MinVersion < tls.VersionTLS12 || cloned.InsecureSkipVerify {
		return nil, ErrSIEMTLSWeak
	}
	if cloned.ServerName == "" {
		cloned.ServerName = host
	}
	return &CEFClient{address: address, tlsConfig: cloned}, nil
}

func (c *CEFClient) Provider() string { return "cef-tls" }

func (c *CEFClient) Publish(ctx context.Context, event audit.Event) error {
	payload, err := EncodeCEF(event)
	if err != nil {
		return err
	}
	if len(payload) > maxCEFEventBytes {
		return fmt.Errorf("siem CEF event exceeds %d bytes", maxCEFEventBytes)
	}
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: c.tlsConfig}
	connection, err := dialer.DialContext(ctx, "tcp", c.address)
	if err != nil {
		return fmt.Errorf("dial SIEM endpoint: %w", err)
	}
	defer func() { _ = connection.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetWriteDeadline(deadline)
	} else {
		_ = connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
	}
	frame := []byte(strconv.Itoa(len(payload)) + " " + payload)
	for written := 0; written < len(frame); {
		count, writeErr := connection.Write(frame[written:])
		if writeErr != nil {
			return fmt.Errorf("write SIEM event: %w", writeErr)
		}
		if count <= 0 {
			return errors.New("write SIEM event made no progress")
		}
		written += count
	}
	return nil
}

// EncodeCEF emits only stable audit metadata. Details are represented by a
// digest so arbitrary business payloads cannot be copied into the SIEM stream.
func EncodeCEF(event audit.Event) (string, error) {
	if strings.TrimSpace(event.Action) == "" {
		return "", errors.New("audit action is required for CEF")
	}
	details, err := json.Marshal(event.Details)
	if err != nil {
		return "", fmt.Errorf("encode audit details for CEF: %w", err)
	}
	detailsDigest := sha256.Sum256(details)
	severity := cefSeverity(event.Result)
	signature := event.Action
	if strings.TrimSpace(event.ResourceType) != "" {
		signature = event.ResourceType + "." + event.Action
	}
	header := strings.Join([]string{
		"CEF:0", "Forge", "Forge", "1", escapeCEFHeader(signature), escapeCEFHeader(event.Action), severity,
	}, "|")
	extensions := make([]string, 0, 11)
	appendCEFExtension(&extensions, "request_id", event.RequestID)
	appendCEFExtension(&extensions, "organization_id", event.OrganizationID)
	appendCEFExtension(&extensions, "actor_id", event.ActorID)
	appendCEFExtension(&extensions, "resource_type", event.ResourceType)
	appendCEFExtension(&extensions, "resource_id", event.ResourceID)
	appendCEFExtension(&extensions, "result", event.Result)
	if !event.OccurredAt.IsZero() {
		appendCEFExtension(&extensions, "rt", strconv.FormatInt(event.OccurredAt.UnixMilli(), 10))
	}
	appendCEFExtension(&extensions, "sequence", strconv.FormatInt(event.SequenceNo, 10))
	appendCEFExtension(&extensions, "event_hash", event.EventHash)
	appendCEFExtension(&extensions, "details_hash", fmt.Sprintf("%x", detailsDigest[:]))
	return header + "|" + strings.Join(extensions, " "), nil
}

func appendCEFExtension(extensions *[]string, key, value string) {
	if value == "" {
		return
	}
	*extensions = append(*extensions, key+"="+escapeCEFExtension(value))
}

func escapeCEFHeader(value string) string {
	return strings.NewReplacer(`\`, `\\`, `|`, `\|`, "\n", `\n`, "\r", `\r`).Replace(value)
}

func escapeCEFExtension(value string) string {
	return strings.NewReplacer(`\`, `\\`, "=", `\=`, "\n", `\n`, "\r", `\r`).Replace(value)
}

func cefSeverity(result string) string {
	switch strings.ToUpper(strings.TrimSpace(result)) {
	case "FAILURE", "ERROR":
		return "8"
	case "DENIED":
		return "7"
	default:
		return "5"
	}
}
