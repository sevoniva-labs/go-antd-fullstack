package nacosx

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
)

// ClientSettings is shared by Nacos config-center and service-registry adapters.
type ClientSettings struct {
	Servers   []string
	Namespace string
	Username  string
	Password  string
	LogLevel  string
}

func Build(settings ClientSettings) (constant.ClientConfig, []constant.ServerConfig, error) {
	if len(settings.Servers) == 0 {
		return constant.ClientConfig{}, nil, fmt.Errorf("nacos servers required")
	}
	cc := constant.ClientConfig{
		NamespaceId:          settings.Namespace,
		TimeoutMs:            5000,
		NotLoadCacheAtStart:  false,
		UpdateCacheWhenEmpty: false,
		Username:             settings.Username,
		Password:             settings.Password,
		LogDir:               "/tmp/forge-nacos/log",
		CacheDir:             "/tmp/forge-nacos/cache",
		LogLevel:             defaultString(settings.LogLevel, "warn"),
	}
	servers := make([]constant.ServerConfig, 0, len(settings.Servers))
	for _, raw := range settings.Servers {
		sc, err := parseServer(raw)
		if err != nil {
			return constant.ClientConfig{}, nil, err
		}
		servers = append(servers, sc)
	}
	return cc, servers, nil
}

func parseServer(raw string) (constant.ServerConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return constant.ServerConfig{}, fmt.Errorf("empty nacos server")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return constant.ServerConfig{}, fmt.Errorf("nacos server %q: %w", raw, err)
	}
	host := u.Hostname()
	if host == "" {
		return constant.ServerConfig{}, fmt.Errorf("nacos server %q missing host", raw)
	}
	port := uint64(8848)
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return constant.ServerConfig{}, fmt.Errorf("nacos server %q invalid port", raw)
		}
		port = n
	}
	path := strings.TrimSpace(u.Path)
	if path == "/" {
		path = ""
	}
	return constant.ServerConfig{IpAddr: host, Port: port, Scheme: defaultString(u.Scheme, "http"), ContextPath: path}, nil
}

func defaultString(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
