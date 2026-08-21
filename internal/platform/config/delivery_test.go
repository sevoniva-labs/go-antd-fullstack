package config

import "testing"

func TestDeliveryValidationRequiresReliableTopics(t *testing.T) {
	cfg := Default()
	cfg.Notification.Provider = "smtp"
	cfg.Notification.SMTPAddress = "smtp.example.com:465"
	cfg.Notification.SMTPUsername = "user"
	cfg.Notification.SMTPPassword = "password"
	cfg.Messaging.Provider = "rocketmq"
	cfg.Messaging.RocketMQTopics = []string{"other.topic"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected notification topic validation error")
	}
}

func TestDeliveryValidationAcceptsConfiguredTopics(t *testing.T) {
	cfg := Default()
	cfg.Notification.Provider = "smtp"
	cfg.Notification.SMTPAddress = "smtp.example.com:465"
	cfg.Notification.SMTPUsername = "user"
	cfg.Notification.SMTPPassword = "password"
	cfg.Messaging.Provider = "rocketmq"
	cfg.Database.DSN = "postgres://forge:secret@db.example.com:5432/forge"
	cfg.Messaging.RocketMQAccessKey = "access-key"
	cfg.Messaging.RocketMQSecretKey = "secret-key"
	cfg.Messaging.RocketMQEndpoint = "rocketmq.example.com:8081"
	cfg.Messaging.RocketMQGroup = "forge-worker"
	cfg.Messaging.RocketMQTopics = []string{cfg.Notification.Topic}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid delivery config, got %v", err)
	}
}
