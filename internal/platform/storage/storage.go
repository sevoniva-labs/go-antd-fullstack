package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appcfg "github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/tlsx"
)

type Store interface {
	Put(context.Context, string, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Ping(context.Context) error
	Provider() string
}

type Capability string

const (
	CapabilityBasicObjectIO       Capability = "basic_object_io"
	CapabilityMultipartRecovery   Capability = "multipart_recovery"
	CapabilityChecksum            Capability = "checksum"
	CapabilitySSES3               Capability = "sse_s3"
	CapabilitySSEKMS              Capability = "sse_kms"
	CapabilityVersioning          Capability = "versioning"
	CapabilityObjectLock          Capability = "object_lock"
	CapabilityRetention           Capability = "retention"
	CapabilityLegalHold           Capability = "legal_hold"
	CapabilityConstrainedPresign  Capability = "constrained_presign"
	CapabilityTemporaryCredential Capability = "temporary_credentials"
)

type CapabilityState string

const (
	CapabilitySupported   CapabilityState = "supported"
	CapabilityUnsupported CapabilityState = "unsupported"
	CapabilityUnknown     CapabilityState = "unknown"
)

type CapabilityStatus struct {
	State    CapabilityState
	Evidence string
}

type CapabilityReporter interface {
	Capabilities() map[Capability]CapabilityStatus
}

// RequireCapabilities fails closed. A provider must be contract-tested before
// an operation can rely on an advanced S3 feature.
func RequireCapabilities(store Store, required ...Capability) error {
	reporter, ok := store.(CapabilityReporter)
	if !ok {
		return errors.New("storage provider does not report capabilities")
	}
	capabilities := reporter.Capabilities()
	for _, capability := range required {
		status, present := capabilities[capability]
		if !present || status.State != CapabilitySupported {
			return fmt.Errorf("storage capability %q is not verified", capability)
		}
	}
	return nil
}

func New(ctx context.Context, c appcfg.Storage) (Store, error) {
	switch normalizeStorageProvider(c.Provider) {
	case "local", "":
		if err := os.MkdirAll(c.LocalRoot, 0o750); err != nil {
			return nil, err
		}
		return &local{root: c.LocalRoot}, nil
	case "s3":
		return newS3(ctx, c)
	default:
		return nil, errors.New("unsupported storage provider")
	}
}

type local struct{ root string }

func (l *local) path(key string) (string, error) {
	clean := filepath.Clean(key)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid key")
	}
	return clean, nil
}
func (l *local) Put(_ context.Context, key string, r io.Reader) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	root, e := os.OpenRoot(l.root)
	if e != nil {
		return e
	}
	defer func() { _ = root.Close() }()
	if e = root.MkdirAll(filepath.Dir(p), 0o750); e != nil {
		return e
	}
	f, e := root.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if e != nil {
		return e
	}
	if _, e = io.Copy(f, r); e != nil {
		_ = f.Close()
		return e
	}
	return f.Close()
}
func (l *local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, e := l.path(key)
	if e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(l.root)
	if e != nil {
		return nil, e
	}
	f, e := root.Open(p)
	closeErr := root.Close()
	if e != nil {
		return nil, e
	}
	if closeErr != nil {
		_ = f.Close()
		return nil, closeErr
	}
	return f, nil
}
func (l *local) Delete(_ context.Context, key string) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	root, e := os.OpenRoot(l.root)
	if e != nil {
		return e
	}
	defer func() { _ = root.Close() }()
	return root.Remove(p)
}
func (l *local) Ping(context.Context) error { return nil }
func (l *local) Provider() string           { return "local" }
func (l *local) Capabilities() map[Capability]CapabilityStatus {
	return map[Capability]CapabilityStatus{
		CapabilityBasicObjectIO:     {State: CapabilitySupported, Evidence: "local filesystem contract"},
		CapabilityMultipartRecovery: {State: CapabilityUnsupported, Evidence: "local provider has no multipart protocol"},
	}
}

type s3Store struct {
	client *s3.Client
	bucket string
}

func normalizeStorageProvider(value string) string {
	p := strings.ToLower(strings.TrimSpace(value))
	if p == "" {
		return "local"
	}
	switch p {
	case "s3", "s3-compatible", "s3_compatible", "minio", "minio-s3", "oss", "cos", "ceph", "ceph-rgw", "radosgw":
		return "s3"
	default:
		return p
	}
}

func newS3(ctx context.Context, c appcfg.Storage) (Store, error) {
	if c.TLS && strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.Endpoint)), "http://") {
		return nil, errors.New("storage tls=true requires https endpoint")
	}
	tlsCfg, err := tlsx.ClientConfig(tlsx.ClientOptions{
		Enabled: c.TLS, CAFile: c.TLSCAFile, CertFile: c.TLSCertFile, KeyFile: c.TLSKeyFile, ServerName: c.TLSServerName,
	})
	if err != nil {
		return nil, err
	}
	opts := []func(*config.LoadOptions) error{config.WithRegion(c.Region)}
	if tlsCfg != nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = tlsCfg
		opts = append(opts, config.WithHTTPClient(&http.Client{Transport: tr}))
	}
	if c.AccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, "")))
	}
	cfg, e := config.LoadDefaultConfig(ctx, opts...)
	if e != nil {
		return nil, e
	}
	cli := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = c.PathStyle
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
	})
	return &s3Store{client: cli, bucket: c.Bucket}, nil
}
func (s *s3Store) Put(ctx context.Context, key string, r io.Reader) error {
	_, e := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: r})
	return e
}
func (s *s3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	o, e := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if e != nil {
		return nil, e
	}
	return o.Body, nil
}
func (s *s3Store) Delete(ctx context.Context, key string) error {
	_, e := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return e
}
func (s *s3Store) Ping(ctx context.Context) error {
	_, e := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	if e != nil {
		return fmt.Errorf("s3 head bucket: %w", e)
	}
	return nil
}
func (s *s3Store) Provider() string { return "s3" }
func (s *s3Store) Capabilities() map[Capability]CapabilityStatus {
	return map[Capability]CapabilityStatus{
		CapabilityBasicObjectIO:       {State: CapabilitySupported, Evidence: "S3 API PutObject/GetObject/DeleteObject/HeadBucket"},
		CapabilityMultipartRecovery:   {State: CapabilityUnknown, Evidence: "target contract test required"},
		CapabilityChecksum:            {State: CapabilityUnknown, Evidence: "target contract test required"},
		CapabilitySSES3:               {State: CapabilityUnknown, Evidence: "target contract test required"},
		CapabilitySSEKMS:              {State: CapabilityUnknown, Evidence: "target KMS contract test required"},
		CapabilityVersioning:          {State: CapabilityUnknown, Evidence: "target contract test required"},
		CapabilityObjectLock:          {State: CapabilityUnknown, Evidence: "target object-lock contract test required"},
		CapabilityRetention:           {State: CapabilityUnknown, Evidence: "target retention contract test required"},
		CapabilityLegalHold:           {State: CapabilityUnknown, Evidence: "target legal-hold contract test required"},
		CapabilityConstrainedPresign:  {State: CapabilityUnknown, Evidence: "target presign contract test required"},
		CapabilityTemporaryCredential: {State: CapabilityUnknown, Evidence: "target STS contract test required"},
	}
}
