package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sevoniva-labs/forge/internal/adapters/auditsink"
	"github.com/sevoniva-labs/forge/internal/adapters/mailer"
	"github.com/sevoniva-labs/forge/internal/adapters/notificationdelivery"
	"github.com/sevoniva-labs/forge/internal/adapters/repository"
	"github.com/sevoniva-labs/forge/internal/app/audit"
	"github.com/sevoniva-labs/forge/internal/app/notification"
	"github.com/sevoniva-labs/forge/internal/platform/cache"
	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/database"
	"github.com/sevoniva-labs/forge/internal/platform/idempotency"
	"github.com/sevoniva-labs/forge/internal/platform/lock"
	"github.com/sevoniva-labs/forge/internal/platform/logx"
	"github.com/sevoniva-labs/forge/internal/platform/messageworker"
	"github.com/sevoniva-labs/forge/internal/platform/messaging"
	"github.com/sevoniva-labs/forge/internal/platform/reliablemsg"
	"github.com/sevoniva-labs/forge/internal/platform/scheduler"
	securitydatapolicy "github.com/sevoniva-labs/forge/internal/platform/security/datapolicy"
	"github.com/sevoniva-labs/forge/internal/platform/storage"
	"github.com/sevoniva-labs/forge/internal/platform/tlsx"
)

var version = "0.2.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("worker stopped", "err", err)
		os.Exit(1)
	}
}
func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logx.New(cfg.Observability.LogLevel, cfg.Observability.LogFormat, cfg.App.Name+"-worker", cfg.App.Environment, version)
	slog.SetDefault(log)
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	objectStore, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		return err
	}
	cleanupController, err := securitydatapolicy.NewDownloadController(repository.NewDownloadArtifactRepo(db), objectStore, 0)
	if err != nil {
		return err
	}
	lockConfig := cfg.Cache
	if lockConfig.Provider == "disabled" {
		lockConfig.Provider = "memory"
	}
	lockCache, err := cache.New(lockConfig)
	if err != nil {
		return err
	}
	defer func() { _ = lockCache.Close() }()
	cleanupRunner := scheduler.New(lock.New(lockCache), log)
	go cleanupRunner.RunDistributed(ctx, "governed-export-cleanup", time.Minute, 2*time.Minute, func(jobCtx context.Context) error {
		expired, expireErr := cleanupController.ExpirePending(jobCtx, 100)
		if expired > 0 {
			log.Info("governed export tickets expired", "count", expired)
		}
		cleaned, cleanupErr := cleanupController.CleanupPending(jobCtx, 100)
		if cleaned > 0 {
			log.Info("governed export objects cleaned", "count", cleaned)
		}
		if expireErr != nil {
			return expireErr
		}
		return cleanupErr
	})
	bus, err := messaging.New(cfg.Messaging)
	if err != nil {
		return err
	}
	defer bus.Close()
	if bus.Provider() == "disabled" {
		log.Info("reliable-message worker disabled because messaging provider is disabled")
		<-ctx.Done()
		return nil
	}
	deliveryRunner, err := newDeliveryRunner(cfg, db, log)
	if err != nil {
		return err
	}
	deliveryErrors := make(chan error, 1)
	if deliveryRunner != nil {
		go func() { deliveryErrors <- deliveryRunner.Run(ctx) }()
	}
	messages := reliablemsg.New(db)
	idem := idempotency.New(db)
	auditWriter := audit.NewWriter(db)
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	gc := time.NewTicker(time.Hour)
	defer gc.Stop()
	auditGC := time.NewTicker(24 * time.Hour)
	defer auditGC.Stop()
	runAuditRetention := func() {
		if cfg.Compliance.AuditRetentionDays <= 0 {
			return
		}
		if n, err := auditWriter.PurgeExpired(ctx, cfg.Compliance.AuditRetentionDays); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("audit log retention gc", "err", err, "retention_days", cfg.Compliance.AuditRetentionDays)
		} else if n > 0 {
			log.Info("audit logs purged", "deleted", n, "retention_days", cfg.Compliance.AuditRetentionDays)
		}
	}
	runAuditRetention()
	if cfg.Compliance.NetworkLogRetentionDays > 0 {
		log.Info("network log retention is enforced by your log platform in this scaffold; app retention config kept for policy control")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-deliveryErrors:
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		case <-gc.C:
			if err := idem.PurgeExpired(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("idempotency gc", "err", err)
			}
		case <-auditGC.C:
			runAuditRetention()
		case <-poll.C:
			n, err := messages.PublishBatch(ctx, bus, 100)
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Error("reliable message publish", "err", err)
			} else if n > 0 {
				log.Info("reliable message published", "count", n)
			}
		}
	}
}

func newDeliveryRunner(cfg config.Config, db *database.DB, log *slog.Logger) (*messageworker.Runner, error) {
	handlers := make(map[string]messageworker.Handler)
	if cfg.Notification.Provider == "smtp" {
		tlsConfig, err := tlsx.ClientConfig(tlsx.ClientOptions{
			Enabled: true, CAFile: cfg.Notification.SMTPCACertFile,
			CertFile: cfg.Notification.SMTPCertFile, KeyFile: cfg.Notification.SMTPKeyFile,
			ServerName: cfg.Notification.SMTPTLSServer,
		})
		if err != nil {
			return nil, err
		}
		sender, err := mailer.NewSMTPClient(mailer.Config{
			Address: cfg.Notification.SMTPAddress, Username: cfg.Notification.SMTPUsername,
			Password: cfg.Notification.SMTPPassword, TLSConfig: tlsConfig,
			TLSMode: mailer.TLSMode(cfg.Notification.SMTPTLSMode),
		})
		if err != nil {
			return nil, err
		}
		handler, err := notificationdelivery.NewEmailHandler(sender)
		if err != nil {
			return nil, err
		}
		handlers[notification.EmailMessageType] = handler.Handle
	}
	if cfg.SIEM.Provider == "cef-tls" {
		tlsConfig, err := tlsx.ClientConfig(tlsx.ClientOptions{
			Enabled: true, CAFile: cfg.SIEM.TLSCAFile,
			CertFile: cfg.SIEM.TLSCertFile, KeyFile: cfg.SIEM.TLSKeyFile,
			ServerName: cfg.SIEM.TLSServerName,
		})
		if err != nil {
			return nil, err
		}
		sink, err := auditsink.NewCEFClient(cfg.SIEM.Address, tlsConfig)
		if err != nil {
			return nil, err
		}
		handler, err := auditsink.NewEventHandler(sink)
		if err != nil {
			return nil, err
		}
		handlers["audit.event"] = handler.Handle
	}
	if len(handlers) == 0 {
		return nil, nil
	}
	consumer, err := messaging.NewConsumer(cfg.Messaging)
	if err != nil {
		return nil, err
	}
	return messageworker.New(db, consumer, handlers, log)
}
