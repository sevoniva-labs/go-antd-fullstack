package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProductionProfilesAreSecureAndValid(t *testing.T) {
	profiles := []string{"standard.yaml", "full.yaml", "xinchuang.yaml"}
	for _, profile := range profiles {
		t.Run(profile, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "configs", profile))
			if err != nil {
				t.Fatalf("read profile: %v", err)
			}
			cfg := Default()
			if err := yaml.Unmarshal(raw, &cfg); err != nil {
				t.Fatalf("parse profile: %v", err)
			}
			switch cfg.Database.Provider {
			case "postgres":
				cfg.Database.DSN = "postgres://forge:secret@db/forge?sslmode=verify-full"
			case "oceanbase":
				cfg.Database.DSN = "forge:secret@tcp(oceanbase:2881)/forge?tls=true"
			}
			if cfg.Messaging.Provider == "rocketmq" {
				cfg.Messaging.RocketMQAccessKey = "access-key"
				cfg.Messaging.RocketMQSecretKey = "secret-key"
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("production profile rejected: %v", err)
			}
			if profile == "full.yaml" && (cfg.Messaging.Provider != "rocketmq" || cfg.Streaming.Provider != "disabled") {
				t.Fatalf("full profile messaging=%q streaming=%q", cfg.Messaging.Provider, cfg.Streaming.Provider)
			}
		})
	}
}
