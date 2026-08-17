package discovery

import (
	"context"
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
		service := cfg.ServiceName
		if service == "" {
			service = appName
		}
		meta := map[string]string{"application": appName, "version": version, "environment": env}
		for k, v := range cfg.Metadata {
			meta[k] = v
		}
		return &nacosRegistry{client: client, cfg: cfg, service: service, metadata: meta}, nil
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
	service    string
	metadata   map[string]string
	registered atomic.Bool
}

func (n *nacosRegistry) Register(context.Context) error {
	ok, err := n.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip: n.cfg.AdvertiseIP, Port: n.cfg.AdvertisePort, ServiceName: n.service,
		Weight: n.cfg.Weight, Enable: true, Healthy: true, Ephemeral: true,
		Metadata: n.metadata, ClusterName: n.cfg.Cluster, GroupName: n.cfg.Group,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("nacos register returned false")
	}
	n.registered.Store(true)
	return nil
}
func (n *nacosRegistry) Deregister(context.Context) error {
	if !n.registered.Load() {
		return nil
	}
	ok, err := n.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip: n.cfg.AdvertiseIP, Port: n.cfg.AdvertisePort, ServiceName: n.service,
		Ephemeral: true, Cluster: n.cfg.Cluster, GroupName: n.cfg.Group,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("nacos deregister returned false")
	}
	n.registered.Store(false)
	return nil
}
func (n *nacosRegistry) Ping(context.Context) error {
	if !n.registered.Load() {
		return fmt.Errorf("nacos service not registered")
	}
	_, err := n.client.GetService(vo.GetServiceParam{
		ServiceName: n.service, Clusters: []string{n.cfg.Cluster}, GroupName: n.cfg.Group,
	})
	return err
}
func (n *nacosRegistry) Provider() string { return "nacos" }

func MetadataPort(port uint64) string { return strconv.FormatUint(port, 10) }
