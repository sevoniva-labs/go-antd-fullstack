package messaging

import (
	"context"
	"errors"
	"time"

	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/tlsx"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
)

type Bus interface {
	Publish(context.Context, string, []byte, []byte) error
	Ping(context.Context) error
	Close()
	Provider() string
}

func New(cfg config.Messaging) (Bus, error) {
	switch cfg.Provider {
	case "disabled", "":
		return noop{}, nil
	case "kafka":
		opts := []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...), kgo.ClientID(cfg.ClientID)}
		tlsCfg, err := tlsx.ClientConfig(tlsx.ClientOptions{
			Enabled: cfg.TLS, CAFile: cfg.TLSCAFile, CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, ServerName: cfg.TLSServerName,
		})
		if err != nil {
			return nil, err
		}
		if tlsCfg != nil {
			opts = append(opts, kgo.DialTLSConfig(tlsCfg))
		}
		if cfg.Username != "" {
			opts = append(opts, kgo.SASL(plain.Auth{User: cfg.Username, Pass: cfg.Password}.AsMechanism()))
		}
		c, err := kgo.NewClient(opts...)
		if err != nil {
			return nil, err
		}
		return &kafka{client: c}, nil
	case "rocketmq":
		return nil, errors.New("rocketmq is an optional official-SDK adapter; see integrations/rocketmq")
	default:
		return nil, errors.New("unsupported messaging provider")
	}
}

type noop struct{}

func (noop) Publish(context.Context, string, []byte, []byte) error { return nil }
func (noop) Ping(context.Context) error                            { return nil }
func (noop) Close()                                                {}
func (noop) Provider() string                                      { return "disabled" }

type kafka struct{ client *kgo.Client }

func (k *kafka) Publish(ctx context.Context, topic string, key, value []byte) error {
	return k.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: value, Timestamp: time.Now()}).FirstErr()
}
func (k *kafka) Ping(ctx context.Context) error { return k.client.Ping(ctx) }
func (k *kafka) Close()                         { k.client.Close() }
func (k *kafka) Provider() string               { return "kafka" }
