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

func New(ctx context.Context, c appcfg.Storage) (Store, error) {
	switch c.Provider {
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
	clean := filepath.Clean("/" + key)
	if strings.Contains(clean, "..") {
		return "", errors.New("invalid key")
	}
	return filepath.Join(l.root, strings.TrimPrefix(clean, "/")), nil
}
func (l *local) Put(_ context.Context, key string, r io.Reader) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0o750); e != nil {
		return e
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = io.Copy(f, r)
	return e
}
func (l *local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, e := l.path(key)
	if e != nil {
		return nil, e
	}
	return os.Open(p)
}
func (l *local) Delete(_ context.Context, key string) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	return os.Remove(p)
}
func (l *local) Ping(context.Context) error { return nil }
func (l *local) Provider() string           { return "local" }

type s3Store struct {
	client *s3.Client
	bucket string
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
