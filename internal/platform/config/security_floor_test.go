package config

import (
	"strings"
	"testing"
	"time"
)

func TestProductionRejectsWeakSecurityFloors(t *testing.T) {
	tests := map[string]func(*Config){
		"short password":           func(c *Config) { c.Security.PasswordMinLength = 11 },
		"missing character class":  func(c *Config) { c.Security.PasswordSymbol = false },
		"short password history":   func(c *Config) { c.Security.PasswordHistory = 4 },
		"disabled password expiry": func(c *Config) { c.Security.PasswordMaxAgeDay = 0 },
		"permissive lockout":       func(c *Config) { c.Security.LoginMaxFailures = 6 },
		"short lock duration":      func(c *Config) { c.Security.LoginLockDuration = 14 * time.Minute },
		"long session":             func(c *Config) { c.Security.SessionTTL = 13 * time.Hour },
	}
	for name, weaken := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := productionConfig()
			weaken(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "security.") {
				t.Fatalf("weak production security configuration accepted: %v", err)
			}
		})
	}
}

func TestDevelopmentKeepsExplicitSecurityConfigurationFlexible(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=disable"
	cfg.Security.PasswordMinLength = 8
	cfg.Security.PasswordUpper = false
	cfg.Security.PasswordLower = false
	cfg.Security.PasswordDigit = false
	cfg.Security.PasswordSymbol = false
	cfg.Security.PasswordHistory = 0
	cfg.Security.PasswordMaxAgeDay = 0
	cfg.Security.LoginMaxFailures = 0
	cfg.Security.LoginLockDuration = 0
	cfg.Security.SessionTTL = 0

	if err := cfg.Validate(); err != nil {
		t.Fatalf("development security configuration rejected: %v", err)
	}
}
