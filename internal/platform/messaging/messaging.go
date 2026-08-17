package messaging

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/tlsx"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
)

type Bus interface {
	Publish(context.Context, Message) (Receipt, error)
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
		return newRocketMQ(cfg)
	default:
		return nil, fmt.Errorf("unsupported messaging provider %q", cfg.Provider)
	}
}

type noop struct{}

func (noop) Publish(context.Context, Message) (Receipt, error) {
	return Receipt{}, errors.New("messaging provider is disabled")
}
func (noop) Ping(context.Context) error { return nil }
func (noop) Close()                     {}
func (noop) Provider() string           { return "disabled" }

type kafka struct{ client *kgo.Client }

func (k *kafka) Publish(ctx context.Context, message Message) (Receipt, error) {
	message, headers, err := prepareMessage(ctx, message)
	if err != nil {
		return Receipt{}, err
	}
	if message.DeliverAt.After(time.Now()) {
		return Receipt{}, errors.New("kafka stream provider does not support delayed business messages")
	}
	key := message.Key
	if message.OrderingKey != "" {
		key = []byte(message.OrderingKey)
	}
	headerNames := make([]string, 0, len(headers))
	for name := range headers {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)
	recordHeaders := make([]kgo.RecordHeader, 0, len(headers))
	for _, name := range headerNames {
		recordHeaders = append(recordHeaders, kgo.RecordHeader{Key: name, Value: []byte(headers[name])})
	}
	err = k.client.ProduceSync(ctx, &kgo.Record{
		Topic: message.Topic, Key: key, Value: message.Body, Headers: recordHeaders, Timestamp: time.Now().UTC(),
	}).FirstErr()
	if err != nil {
		return Receipt{}, fmt.Errorf("publish kafka topic %q: %w", message.Topic, err)
	}
	return Receipt{Provider: "kafka"}, nil
}
func (k *kafka) Ping(ctx context.Context) error { return k.client.Ping(ctx) }
func (k *kafka) Close()                         { k.client.Close() }
func (k *kafka) Provider() string               { return "kafka" }
