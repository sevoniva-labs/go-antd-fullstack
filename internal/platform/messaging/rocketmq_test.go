package messaging

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/sevoniva-labs/forge/internal/platform/config"
)

type fakeRocketMQProducer struct {
	startErr error
	sendErr  error
	message  *rmq.Message
	started  int
	stopped  int
}

func (f *fakeRocketMQProducer) Send(_ context.Context, message *rmq.Message) ([]*rmq.SendReceipt, error) {
	f.message = message
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return []*rmq.SendReceipt{{MessageID: "01HROCKETMQ"}}, nil
}
func (f *fakeRocketMQProducer) Start() error {
	f.started++
	return f.startErr
}
func (f *fakeRocketMQProducer) GracefulStop() error {
	f.stopped++
	return nil
}

func TestRocketMQStartsWithVerifiedTLSAndPublishesAllowedTopic(t *testing.T) {
	fake := &fakeRocketMQProducer{}
	cfg := config.Messaging{
		TLS:               true,
		TLSServerName:     "rocketmq.internal.example",
		RocketMQTopics:    []string{"audit-events", "audit-events"},
		RocketMQEndpoint:  "rocketmq-proxy:8081",
		RocketMQAccessKey: "access",
		RocketMQSecretKey: "secret",
	}
	bus, err := newRocketMQWithFactory(cfg, func(_ config.Messaging, tlsConfig *tls.Config, topics []string) (rocketMQProducer, error) {
		if tlsConfig == nil || tlsConfig.InsecureSkipVerify {
			t.Fatal("RocketMQ TLS must verify the server certificate")
		}
		if tlsConfig.ServerName != cfg.TLSServerName {
			t.Fatalf("TLS server name = %q", tlsConfig.ServerName)
		}
		if len(topics) != 1 || topics[0] != "audit-events" {
			t.Fatalf("normalized topics = %v", topics)
		}
		return fake, nil
	})
	if err != nil {
		t.Fatalf("newRocketMQWithFactory() error = %v", err)
	}
	if err := bus.Publish(context.Background(), "audit-events", []byte("event-1"), []byte(`{"action":"login"}`)); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if fake.started != 1 || fake.message == nil {
		t.Fatalf("producer started = %d, message = %#v", fake.started, fake.message)
	}
	if got := fake.message.GetKeys(); len(got) != 1 || got[0] != "event-1" {
		t.Fatalf("message keys = %v", got)
	}
	bus.Close()
	bus.Close()
	if fake.stopped != 1 {
		t.Fatalf("GracefulStop() calls = %d, want 1", fake.stopped)
	}
}

func TestRocketMQRejectsUnconfiguredTopic(t *testing.T) {
	fake := &fakeRocketMQProducer{}
	bus, err := newRocketMQWithFactory(config.Messaging{RocketMQTopics: []string{"allowed"}}, func(config.Messaging, *tls.Config, []string) (rocketMQProducer, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatalf("newRocketMQWithFactory() error = %v", err)
	}
	defer bus.Close()
	if err := bus.Publish(context.Background(), "other", nil, []byte("payload")); err == nil {
		t.Fatal("Publish() accepted an unconfigured topic")
	}
	if fake.message != nil {
		t.Fatal("producer received a rejected message")
	}
}

func TestRocketMQCleansUpAfterStartupFailure(t *testing.T) {
	fake := &fakeRocketMQProducer{startErr: errors.New("proxy unavailable")}
	_, err := newRocketMQWithFactory(config.Messaging{RocketMQTopics: []string{"events"}}, func(config.Messaging, *tls.Config, []string) (rocketMQProducer, error) {
		return fake, nil
	})
	if err == nil {
		t.Fatal("newRocketMQWithFactory() accepted startup failure")
	}
	if fake.stopped != 1 {
		t.Fatalf("GracefulStop() calls = %d, want 1", fake.stopped)
	}
}
