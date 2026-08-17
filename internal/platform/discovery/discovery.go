package discovery

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/sevoniva-labs/forge/internal/platform/config"
	"github.com/sevoniva-labs/forge/internal/platform/nacosx"
)

type Registry interface {
	Register(context.Context) error
	Deregister(context.Context) error
	Ping(context.Context) error
	Provider() string
}

func New(cfg config.Discovery, appName, version, env string) (Registry, error) {
	switch cfg.Provider {
	case "", "disabled":
		return disabled{}, nil
	case "nacos":
		cc, servers, err := nacosx.Build(nacosx.ClientSettings{
			Servers: cfg.Servers, Namespace: cfg.Namespace,
			Username: cfg.Username, Password: cfg.Password, LogLevel: "warn",
			TLSRequired: cfg.TLSRequired, TLSCAFile: cfg.TLSCAFile,
			TLSCertFile: cfg.TLSCertFile, TLSKeyFile: cfg.TLSKeyFile, TLSServerName: cfg.TLSServerName,
		})
		if err != nil {
			return nil, err
		}
		client, err := clients.NewNamingClient(vo.NacosClientParam{ClientConfig: &cc, ServerConfigs: servers})
		if err != nil {
			return nil, err
		}
		meta := map[string]string{"application": appName, "version": version, "environment": env}
		for k, v := range cfg.Metadata {
			meta[k] = v
		}
		return &nacosRegistry{client: client, cfg: cfg, endpoints: buildEndpoints(cfg, appName, meta)}, nil
	default:
		return nil, fmt.Errorf("unsupported discovery provider %q", cfg.Provider)
	}
}

type disabled struct{}

func (disabled) Register(context.Context) error   { return nil }
func (disabled) Deregister(context.Context) error { return nil }
func (disabled) Ping(context.Context) error       { return nil }
func (disabled) Provider() string                 { return "disabled" }

type nacosRegistry struct {
	client     naming_client.INamingClient
	cfg        config.Discovery
	endpoints  []endpoint
	registered atomic.Bool
}

type endpoint struct {
	service  string
	port     uint64
	metadata map[string]string
}

func buildEndpoints(cfg config.Discovery, appName string, baseMetadata map[string]string) []endpoint {
	httpService := cfg.ServiceName
	if httpService == "" {
		httpService = appName + "-http"
	}
	grpcService := cfg.GRPCServiceName
	if grpcService == "" {
		grpcService = appName + "-grpc"
	}
	makeMetadata := func(protocol string, port uint64) map[string]string {
		metadata := make(map[string]string, len(baseMetadata)+2)
		for key, value := range baseMetadata {
			metadata[key] = value
		}
		metadata["protocol"] = protocol
		metadata["port"] = MetadataPort(port)
		return metadata
	}
	return []endpoint{
		{service: httpService, port: cfg.AdvertisePort, metadata: makeMetadata("http", cfg.AdvertisePort)},
		{service: grpcService, port: cfg.AdvertiseGRPCPort, metadata: makeMetadata("grpc", cfg.AdvertiseGRPCPort)},
	}
}

func (n *nacosRegistry) Register(context.Context) error {
	registered := make([]endpoint, 0, len(n.endpoints))
	for _, item := range n.endpoints {
		ok, err := n.client.RegisterInstance(vo.RegisterInstanceParam{
			Ip: n.cfg.AdvertiseIP, Port: item.port, ServiceName: item.service,
			Weight: n.cfg.Weight, Enable: true, Healthy: true, Ephemeral: true,
			Metadata: item.metadata, ClusterName: n.cfg.Cluster, GroupName: n.cfg.Group,
		})
		if err != nil || !ok {
			for index := len(registered) - 1; index >= 0; index-- {
				_ = n.deregisterEndpoint(registered[index])
			}
			if err != nil {
				return fmt.Errorf("register Nacos service %s: %w", item.service, err)
			}
			return fmt.Errorf("register Nacos service %s returned false", item.service)
		}
		registered = append(registered, item)
	}
	n.registered.Store(true)
	return nil
}
func (n *nacosRegistry) Deregister(context.Context) error {
	if !n.registered.Load() {
		return nil
	}
	var errs []error
	for index := len(n.endpoints) - 1; index >= 0; index-- {
		if err := n.deregisterEndpoint(n.endpoints[index]); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		n.registered.Store(false)
	}
	return errors.Join(errs...)
}

func (n *nacosRegistry) deregisterEndpoint(item endpoint) error {
	ok, err := n.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip: n.cfg.AdvertiseIP, Port: item.port, ServiceName: item.service,
		Ephemeral: true, Cluster: n.cfg.Cluster, GroupName: n.cfg.Group,
	})
	if err != nil {
		return fmt.Errorf("deregister Nacos service %s: %w", item.service, err)
	}
	if !ok {
		return fmt.Errorf("deregister Nacos service %s returned false", item.service)
	}
	return nil
}

func (n *nacosRegistry) Ping(context.Context) error {
	if !n.registered.Load() {
		return fmt.Errorf("nacos service not registered")
	}
	for _, item := range n.endpoints {
		if _, err := n.client.GetService(vo.GetServiceParam{
			ServiceName: item.service, Clusters: []string{n.cfg.Cluster}, GroupName: n.cfg.Group,
		}); err != nil {
			return fmt.Errorf("query Nacos service %s: %w", item.service, err)
		}
	}
	return nil
}
func (n *nacosRegistry) Provider() string { return "nacos" }

func MetadataPort(port uint64) string { return strconv.FormatUint(port, 10) }
