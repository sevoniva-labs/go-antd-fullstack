package config

import "strings"

// Notification configures provider-neutral outbound notifications. Secrets are
// intentionally excluded from YAML and must come from the environment.
type Notification struct {
	Provider       string `yaml:"provider"`
	Topic          string `yaml:"topic"`
	SMTPAddress    string `yaml:"smtp_address"`
	SMTPUsername   string `yaml:"-"`
	SMTPPassword   string `yaml:"-"`
	SMTPTLSMode    string `yaml:"smtp_tls_mode"`
	SMTPCACertFile string `yaml:"smtp_tls_ca_file"`
	SMTPCertFile   string `yaml:"smtp_tls_cert_file"`
	SMTPKeyFile    string `yaml:"smtp_tls_key_file"`
	SMTPTLSServer  string `yaml:"smtp_tls_server_name"`
}

// SIEM configures an optional provider-neutral audit event sink.
type SIEM struct {
	Provider      string `yaml:"provider"`
	Topic         string `yaml:"topic"`
	Address       string `yaml:"address"`
	TLSCAFile     string `yaml:"tls_ca_file"`
	TLSCertFile   string `yaml:"tls_cert_file"`
	TLSKeyFile    string `yaml:"tls_key_file"`
	TLSServerName string `yaml:"tls_server_name"`
}

func applyDeliveryEnvironment(cfg *Config) {
	cfg.Notification.SMTPUsername = secret("FORGE_SMTP_USERNAME")
	cfg.Notification.SMTPPassword = secret("FORGE_SMTP_PASSWORD")
	overrideString(&cfg.Notification.Provider, "FORGE_NOTIFICATION_PROVIDER")
	overrideString(&cfg.Notification.Topic, "FORGE_NOTIFICATION_TOPIC")
	overrideString(&cfg.Notification.SMTPAddress, "FORGE_SMTP_ADDRESS")
	overrideString(&cfg.Notification.SMTPTLSMode, "FORGE_SMTP_TLS_MODE")
	overrideString(&cfg.Notification.SMTPCACertFile, "FORGE_SMTP_TLS_CA_FILE")
	overrideString(&cfg.Notification.SMTPCertFile, "FORGE_SMTP_TLS_CERT_FILE")
	overrideString(&cfg.Notification.SMTPKeyFile, "FORGE_SMTP_TLS_KEY_FILE")
	overrideString(&cfg.Notification.SMTPTLSServer, "FORGE_SMTP_TLS_SERVER_NAME")

	overrideString(&cfg.SIEM.Provider, "FORGE_SIEM_PROVIDER")
	overrideString(&cfg.SIEM.Topic, "FORGE_SIEM_TOPIC")
	overrideString(&cfg.SIEM.Address, "FORGE_SIEM_ADDRESS")
	overrideString(&cfg.SIEM.TLSCAFile, "FORGE_SIEM_TLS_CA_FILE")
	overrideString(&cfg.SIEM.TLSCertFile, "FORGE_SIEM_TLS_CERT_FILE")
	overrideString(&cfg.SIEM.TLSKeyFile, "FORGE_SIEM_TLS_KEY_FILE")
	overrideString(&cfg.SIEM.TLSServerName, "FORGE_SIEM_TLS_SERVER_NAME")
}

func validateDelivery(c Config) []string {
	var errs []string
	if c.Notification.Provider != "disabled" && c.Notification.Provider != "smtp" {
		errs = append(errs, "notification.provider must be disabled|smtp")
	}
	if c.SIEM.Provider != "disabled" && c.SIEM.Provider != "cef-tls" {
		errs = append(errs, "siem.provider must be disabled|cef-tls")
	}
	if c.Notification.Provider == "smtp" {
		if strings.TrimSpace(c.Notification.SMTPAddress) == "" || strings.TrimSpace(c.Notification.Topic) == "" {
			errs = append(errs, "notification smtp requires smtp_address and topic")
		}
		if c.Notification.SMTPTLSMode != "starttls" && c.Notification.SMTPTLSMode != "implicit" {
			errs = append(errs, "notification.smtp_tls_mode must be starttls|implicit")
		}
		if (c.Notification.SMTPCertFile == "") != (c.Notification.SMTPKeyFile == "") {
			errs = append(errs, "notification smtp tls cert and key must be configured together")
		}
	}
	if c.SIEM.Provider == "cef-tls" {
		if strings.TrimSpace(c.SIEM.Address) == "" || strings.TrimSpace(c.SIEM.Topic) == "" {
			errs = append(errs, "siem cef-tls requires address and topic")
		}
		if (c.SIEM.TLSCertFile == "") != (c.SIEM.TLSKeyFile == "") {
			errs = append(errs, "siem tls cert and key must be configured together")
		}
	}
	if (c.Notification.Provider == "smtp" || c.SIEM.Provider == "cef-tls") && c.Messaging.Provider != "rocketmq" {
		errs = append(errs, "enabled notification or siem delivery requires messaging.provider=rocketmq")
	}
	if isProduction(c.App.Environment) {
		if c.Notification.Provider == "smtp" && c.Notification.SMTPTLSMode == "starttls" && strings.TrimSpace(c.Notification.SMTPCACertFile) == "" {
			errs = append(errs, "notification smtp requires smtp_tls_ca_file in production")
		}
		if c.SIEM.Provider == "cef-tls" && strings.TrimSpace(c.SIEM.TLSCAFile) == "" {
			errs = append(errs, "siem requires tls_ca_file in production")
		}
	}
	return errs
}
