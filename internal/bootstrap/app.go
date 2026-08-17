package bootstrap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	ktracing "github.com/go-kratos/kratos/v2/middleware/tracing"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	forgev1 "github.com/sevoniva-labs/forge/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/forge/internal/adapters/kratosapi"
	"github.com/sevoniva-labs/forge/internal/adapters/repository"
	"github.com/sevoniva-labs/forge/internal/app/audit"
	appidentity "github.com/sevoniva-labs/forge/internal/app/identity"
	"github.com/sevoniva-labs/forge/internal/platform/authn"
	"github.com/sevoniva-labs/forge/internal/platform/authz"
	"github.com/sevoniva-labs/forge/internal/platform/cache"
	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/csrf"
	"github.com/sevoniva-labs/forge/internal/platform/database"
	"github.com/sevoniva-labs/forge/internal/platform/discovery"
	"github.com/sevoniva-labs/forge/internal/platform/health"
	"github.com/sevoniva-labs/forge/internal/platform/httpserver"
	"github.com/sevoniva-labs/forge/internal/platform/logx"
	"github.com/sevoniva-labs/forge/internal/platform/messaging"
	"github.com/sevoniva-labs/forge/internal/platform/metrics"
	"github.com/sevoniva-labs/forge/internal/platform/observability"
	"github.com/sevoniva-labs/forge/internal/platform/ratelimit"
	"github.com/sevoniva-labs/forge/internal/platform/remoteconfig"
	"github.com/sevoniva-labs/forge/internal/platform/search"
	"github.com/sevoniva-labs/forge/internal/platform/securefile"
	appcrypto "github.com/sevoniva-labs/forge/internal/platform/security/crypto"
	"github.com/sevoniva-labs/forge/internal/platform/storage"
)

type Options struct{ Version string }

type App struct {
	cfg           config.Config
	log           *slog.Logger
	runtime       *kratos.App
	db            *database.DB
	cache         cache.Cache
	bus           messaging.Bus
	registry      discovery.Registry
	traceShutdown observability.Shutdown
}

func New(ctx context.Context, opts Options) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Bootstrap remote config before constructing infrastructure. Environment
	// and *_FILE values are re-applied after the remote YAML so secrets and
	// emergency deployment overrides always win.
	if cfg.RemoteConfig.Provider != "" && cfg.RemoteConfig.Provider != "disabled" {
		src, err := remoteconfig.New(cfg.RemoteConfig)
		if err != nil {
			if cfg.RemoteConfig.FailFast {
				return nil, fmt.Errorf("remote config: %w", err)
			}
			slog.Warn("remote config client unavailable; using local config", "err", err)
		} else if raw, err := src.Load(ctx); err != nil {
			if cfg.RemoteConfig.FailFast {
				return nil, fmt.Errorf("remote config load: %w", err)
			}
			slog.Warn("remote config load failed; using local config", "err", err)
		} else if len(raw) > 0 {
			if err := config.MergeYAML(&cfg, raw); err != nil {
				return nil, err
			}
		}
	}

	log := logx.New(cfg.Observability.LogLevel, cfg.Observability.LogFormat, cfg.App.Name, cfg.App.Environment, opts.Version)
	slog.SetDefault(log)

	traceShutdown, err := observability.InitTracing(ctx, cfg.Observability, cfg.App.Name, opts.Version, cfg.App.Environment)
	if err != nil {
		return nil, fmt.Errorf("tracing: %w", err)
	}

	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		_ = traceShutdown(context.Background())
		return nil, err
	}
	if cfg.Database.AutoMigrate {
		if err = database.Migrate(db.DB, cfg.Database.Provider); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	c, err := cache.New(cfg.Cache)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	bus, err := messaging.New(cfg.Messaging)
	if err != nil {
		_ = c.Close()
		_ = db.Close()
		return nil, err
	}
	se, err := search.New(cfg.Search)
	if err != nil {
		bus.Close()
		_ = c.Close()
		_ = db.Close()
		return nil, err
	}
	st, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		bus.Close()
		_ = c.Close()
		_ = db.Close()
		return nil, err
	}
	crypt, err := appcrypto.New(cfg.Security.CryptoProvider, cfg.Security.CryptoKey, cfg.Security.CryptoKeyVersion)
	if err != nil {
		bus.Close()
		_ = c.Close()
		_ = db.Close()
		return nil, fmt.Errorf("crypto: %w", err)
	}

	reg, err := discovery.New(cfg.Discovery, cfg.App.Name, opts.Version, cfg.App.Environment)
	if err != nil {
		bus.Close()
		_ = c.Close()
		_ = db.Close()
		return nil, fmt.Errorf("service discovery: %w", err)
	}

	repo := repository.NewIdentityRepo(db)
	identitySvc := appidentity.NewService(repo, appidentity.Options{
		MinLength:     cfg.Security.PasswordMinLength,
		RequireUpper:  cfg.Security.PasswordUpper,
		RequireLower:  cfg.Security.PasswordLower,
		RequireDigit:  cfg.Security.PasswordDigit,
		RequireSymbol: cfg.Security.PasswordSymbol,
		History:       cfg.Security.PasswordHistory,
		MaxAgeDays:    cfg.Security.PasswordMaxAgeDay,
		SessionTTL:    cfg.Security.SessionTTL,
		MaxFailures:   cfg.Security.LoginMaxFailures,
		LockDuration:  cfg.Security.LoginLockDuration,
		Crypto:        crypt,
	})
	orgKey := env("FORGE_BOOTSTRAP_ORG_KEY", "default")
	orgName := env("FORGE_BOOTSTRAP_ORG_NAME", cfg.App.Name)
	admin := strings.TrimSpace(os.Getenv("FORGE_BOOTSTRAP_ADMIN"))
	adminPass := secret("FORGE_BOOTSTRAP_PASSWORD")
	if admin != "" && adminPass == "" {
		return nil, fmt.Errorf("FORGE_BOOTSTRAP_PASSWORD is required when FORGE_BOOTSTRAP_ADMIN is set")
	}
	if err = identitySvc.Bootstrap(ctx, orgKey, orgName, admin, adminPass); err != nil {
		return nil, fmt.Errorf("identity bootstrap: %w", err)
	}
	if admin == "" {
		log.Warn("no bootstrap administrator configured", "hint", "set FORGE_BOOTSTRAP_ADMIN and FORGE_BOOTSTRAP_PASSWORD")
	}

	auditWriter := audit.NewWriter(db)
	var met *metrics.Metrics
	if cfg.Observability.MetricsEnabled {
		met = metrics.New()
	}

	tlsCfg, err := serverTLSConfig(cfg.Server)
	if err != nil {
		return nil, err
	}
	publicOperation := func(_ context.Context, operation string) bool {
		switch operation {
		case forgev1.OperationSystemServiceHealth, forgev1.OperationSystemServiceReadiness, forgev1.OperationIdentityServiceLogin:
			return false
		default:
			return true
		}
	}
	grpcSecurity := selector.Server(authn.Server(identitySvc), authz.Server(authz.PlatformRules())).Match(publicOperation).Build()
	httpSecurity := selector.Server(authn.Server(identitySvc), csrf.Server(), authz.Server(authz.PlatformRules())).Match(publicOperation).Build()
	httpOpts := []khttp.ServerOption{
		khttp.Address(cfg.Server.ListenAddr), khttp.Timeout(cfg.Server.WriteTimeout), khttp.Middleware(httpSecurity),
		khttp.Filter(httpserver.Filters(httpserver.FilterOptions{
			Log: log, Metrics: met, Secure: cfg.Security.SecureCookies,
			AllowedOrigins: cfg.Security.AllowedOrigins, MaxBodyBytes: cfg.Server.MaxBodyBytes, ServiceName: cfg.App.Name,
		})...),
	}
	grpcOpts := []kgrpc.ServerOption{kgrpc.Address(cfg.Server.GRPCListenAddr), kgrpc.Timeout(cfg.Server.WriteTimeout), kgrpc.Middleware(ktracing.Server(), grpcSecurity)}
	if tlsCfg != nil {
		httpOpts = append(httpOpts, khttp.TLSConfig(tlsCfg.Clone()))
		grpcOpts = append(grpcOpts, kgrpc.TLSConfig(tlsCfg.Clone()))
	}
	httpServer := khttp.NewServer(httpOpts...)
	httpServer.ReadHeaderTimeout = 10 * time.Second
	httpServer.ReadTimeout = cfg.Server.ReadTimeout
	httpServer.WriteTimeout = cfg.Server.WriteTimeout
	httpServer.IdleTimeout = cfg.Server.IdleTimeout

	checks := []health.Check{
		{Name: "database", Provider: cfg.Database.Provider, Ping: db.PingContext},
		{Name: "cache", Provider: c.Provider(), Ping: c.Ping},
		{Name: "messaging", Provider: bus.Provider(), Ping: bus.Ping},
		{Name: "search", Provider: se.Provider(), Ping: se.Ping},
		{Name: "storage", Provider: st.Provider(), Ping: st.Ping},
	}
	providers := map[string]string{
		"database": cfg.Database.Provider, "cache": c.Provider(), "messaging": bus.Provider(),
		"search": se.Provider(), "storage": st.Provider(), "crypto": crypt.Name(),
		"discovery": reg.Provider(), "remote_config": cfg.RemoteConfig.Provider,
	}
	systemService := kratosapi.NewSystemService(cfg, opts.Version, checks, providers)
	platformService := kratosapi.NewPlatformService(identitySvc, auditWriter, db)
	identityService := kratosapi.NewIdentityService(identitySvc, auditWriter, db, ratelimit.New(c), cfg.Security.SecureCookies, cfg.Security.SameSite)
	forgev1.RegisterSystemServiceHTTPServer(httpServer, systemService)
	forgev1.RegisterIdentityServiceHTTPServer(httpServer, identityService)
	forgev1.RegisterPlatformServiceHTTPServer(httpServer, platformService)
	if met != nil {
		httpServer.Handle(cfg.Observability.MetricsPath, met.Handler())
	}
	if !cfg.Compliance.DisableDebugEndpoints {
		httpServer.HandlePrefix("/debug/pprof/", httpserver.DebugHandler())
	}
	httpServer.HandlePrefix("/", httpserver.SPA(httpserver.SPAOptions{
		Root:            cfg.Server.WebDir,
		FrameSources:    cfg.Server.WebCSPFrameSources,
		ConnectSources:  cfg.Server.WebCSPConnectSources,
		WujieCSPEnabled: cfg.Server.WebCSPWujieEnabled,
	}))
	grpcServer := kgrpc.NewServer(grpcOpts...)
	forgev1.RegisterSystemServiceServer(grpcServer, systemService)
	forgev1.RegisterPlatformServiceServer(grpcServer, platformService)
	forgev1.RegisterIdentityServiceServer(grpcServer, identityService)
	runtime := kratos.New(
		kratos.Context(ctx), kratos.Name(cfg.App.Name), kratos.Version(opts.Version),
		kratos.Metadata(map[string]string{"environment": cfg.App.Environment, "region": cfg.App.Region, "zone": cfg.App.Zone}),
		kratos.Server(httpServer, grpcServer), kratos.StopTimeout(cfg.Server.ShutdownTimeout),
	)
	return &App{cfg: cfg, log: log, runtime: runtime, db: db, cache: c, bus: bus, registry: reg, traceShutdown: traceShutdown}, nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.registry.Register(ctx); err != nil {
		return fmt.Errorf("service register: %w", err)
	}
	defer func() { _ = a.registry.Deregister(context.Background()) }()
	a.log.Info("Kratos servers starting",
		"http_addr", a.cfg.Server.ListenAddr, "grpc_addr", a.cfg.Server.GRPCListenAddr,
		"public_url", a.cfg.Server.PublicURL, "database", a.cfg.Database.Provider,
		"cache", a.cache.Provider(), "messaging", a.bus.Provider(),
		"discovery", a.registry.Provider(), "tls", a.cfg.Server.TLSEnabled)
	return a.runtime.Run()
}
func (a *App) Close() {
	if a.runtime != nil {
		_ = a.runtime.Stop()
	}
	if a.registry != nil {
		_ = a.registry.Deregister(context.Background())
	}
	if a.traceShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = a.traceShutdown(ctx)
		cancel()
	}
	if a.bus != nil {
		a.bus.Close()
	}
	if a.cache != nil {
		_ = a.cache.Close()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
}

func serverTLSConfig(cfg config.Server) (*tls.Config, error) {
	if !cfg.TLSEnabled {
		return nil, nil
	}
	certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}
	t := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
	if cfg.RequireClientTLS {
		raw, err := os.ReadFile(cfg.TLSClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("read client CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(raw) {
			return nil, fmt.Errorf("invalid client CA PEM")
		}
		t.ClientCAs = pool
		t.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return t, nil
}

func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func secret(k string) string {
	if p := strings.TrimSpace(os.Getenv(k + "_FILE")); p != "" {
		if b, e := securefile.Read(p); e == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return strings.TrimSpace(os.Getenv(k))
}
