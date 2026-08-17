package messaging

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/tlsx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	rocketMQMaxSendBytes = 4 << 20
	rocketMQMaxRecvBytes = 16 << 20
)

type rocketMQProducer interface {
	Send(context.Context, *rmq.Message) ([]*rmq.SendReceipt, error)
	Start() error
	GracefulStop() error
}

type rocketMQProducerFactory func(config.Messaging, *tls.Config, []string) (rocketMQProducer, error)

type rocketMQ struct {
	producer rocketMQProducer
	topics   map[string]struct{}
	closed   atomic.Bool
}

func newRocketMQ(cfg config.Messaging) (Bus, error) {
	return newRocketMQWithFactory(cfg, newApacheRocketMQProducer)
}

func newRocketMQWithFactory(cfg config.Messaging, factory rocketMQProducerFactory) (*rocketMQ, error) {
	topics, allowed, err := rocketMQTopics(cfg.RocketMQTopics)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := tlsx.ClientConfig(tlsx.ClientOptions{
		Enabled:    cfg.TLS,
		CAFile:     cfg.TLSCAFile,
		CertFile:   cfg.TLSCertFile,
		KeyFile:    cfg.TLSKeyFile,
		ServerName: cfg.TLSServerName,
	})
	if err != nil {
		return nil, fmt.Errorf("rocketmq tls: %w", err)
	}
	producer, err := factory(cfg, tlsConfig, topics)
	if err != nil {
		return nil, fmt.Errorf("create rocketmq producer: %w", err)
	}
	if err := producer.Start(); err != nil {
		_ = producer.GracefulStop()
		return nil, fmt.Errorf("start rocketmq producer: %w", err)
	}
	return &rocketMQ{producer: producer, topics: allowed}, nil
}

func newApacheRocketMQProducer(cfg config.Messaging, tlsConfig *tls.Config, topics []string) (rocketMQProducer, error) {
	clientFactory := func(rmqConfig *rmq.Config, _ ...rmq.ClientOption) (rmq.Client, error) {
		return rmq.NewClient(rmqConfig, rmq.WithClientConnFunc(rocketMQConnFactory(tlsConfig)))
	}
	return rmq.NewProducer(&rmq.Config{
		Endpoint:      strings.TrimSpace(cfg.RocketMQEndpoint),
		NameSpace:     strings.TrimSpace(cfg.RocketMQNamespace),
		ConsumerGroup: strings.TrimSpace(cfg.RocketMQGroup),
		Credentials: &credentials.SessionCredentials{
			AccessKey:    cfg.RocketMQAccessKey,
			AccessSecret: cfg.RocketMQSecretKey,
		},
	}, rmq.WithTopics(topics...), rmq.WithClientFunc(clientFactory))
}

// rocketMQConnFactory replaces the SDK's insecure default TLS configuration.
// Plaintext is allowed only when the scaffold configuration explicitly disables
// TLS; TLS mode always verifies the certificate chain and hostname.
func rocketMQConnFactory(tlsConfig *tls.Config) rmq.ClientConnFunc {
	return func(endpoint string, _ ...rmq.ConnOption) (rmq.ClientConn, error) {
		var transportCredentials grpccredentials.TransportCredentials
		if tlsConfig == nil {
			transportCredentials = insecure.NewCredentials()
		} else {
			transportCredentials = grpccredentials.NewTLS(tlsConfig.Clone())
		}
		conn, err := grpc.NewClient(endpoint,
			grpc.WithTransportCredentials(transportCredentials),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallSendMsgSize(rocketMQMaxSendBytes),
				grpc.MaxCallRecvMsgSize(rocketMQMaxRecvBytes),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("rocketmq grpc client %q: %w", endpoint, err)
		}
		return &rocketMQClientConn{conn: conn}, nil
	}
}

type rocketMQClientConn struct {
	conn *grpc.ClientConn
}

func (c *rocketMQClientConn) Conn() *grpc.ClientConn { return c.conn }
func (c *rocketMQClientConn) Close() error           { return c.conn.Close() }

func (r *rocketMQ) Publish(ctx context.Context, topic string, key, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.closed.Load() {
		return errors.New("rocketmq producer is closed")
	}
	topic = strings.TrimSpace(topic)
	if _, ok := r.topics[topic]; !ok {
		return fmt.Errorf("rocketmq topic %q is not configured", topic)
	}
	message := &rmq.Message{Topic: topic, Body: append([]byte(nil), value...)}
	if len(key) > 0 {
		if utf8.Valid(key) {
			message.SetKeys(string(key))
		} else {
			message.SetKeys(base64.RawURLEncoding.EncodeToString(key))
			message.AddProperty("forge-key-encoding", "base64url")
		}
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for _, name := range carrier.Keys() {
		message.AddProperty(name, carrier.Get(name))
	}
	receipts, err := r.producer.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("publish rocketmq topic %q: %w", topic, err)
	}
	if len(receipts) == 0 {
		return fmt.Errorf("publish rocketmq topic %q: empty send receipt", topic)
	}
	return nil
}

// Start synchronizes producer settings and topic routes with the Proxy. The
// official SDK exposes no public heartbeat, so readiness means that startup
// succeeded and this producer has not been closed.
func (r *rocketMQ) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.closed.Load() {
		return errors.New("rocketmq producer is closed")
	}
	return nil
}

func (r *rocketMQ) Close() {
	if r.closed.CompareAndSwap(false, true) {
		_ = r.producer.GracefulStop()
	}
}

func (r *rocketMQ) Provider() string { return "rocketmq" }

func rocketMQTopics(values []string) ([]string, map[string]struct{}, error) {
	topics := make([]string, 0, len(values))
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		topic := strings.TrimSpace(value)
		if topic == "" {
			continue
		}
		if _, exists := allowed[topic]; exists {
			continue
		}
		allowed[topic] = struct{}{}
		topics = append(topics, topic)
	}
	if len(topics) == 0 {
		return nil, nil, errors.New("rocketmq requires at least one configured topic")
	}
	return topics, allowed, nil
}
