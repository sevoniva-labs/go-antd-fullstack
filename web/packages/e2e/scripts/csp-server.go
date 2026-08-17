package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sevoniva-labs/forge/internal/platform/httpserver"
)

func main() {
	root, err := filepath.Abs("web/dist")
	if err != nil {
		log.Fatal(err)
	}
	address := strings.TrimSpace(os.Getenv("FORGE_E2E_STATIC_ADDR"))
	if address == "" {
		address = "127.0.0.1:4173"
	}
	server := &http.Server{
		Addr: address,
		Handler: httpserver.SPA(httpserver.SPAOptions{
			Root:            root,
			FrameSources:    sourcesFromEnv("FORGE_E2E_FRAME_SOURCES"),
			ConnectSources:  sourcesFromEnv("FORGE_E2E_CONNECT_SOURCES"),
			WujieCSPEnabled: os.Getenv("FORGE_E2E_WUJIE_CSP") == "true",
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("serving CSP test build on http://%s", address)
	log.Fatal(server.ListenAndServe())
}

func sourcesFromEnv(name string) []string {
	sources := make([]string, 0)
	for _, source := range strings.Split(os.Getenv(name), ",") {
		if source = strings.TrimSpace(source); source != "" {
			sources = append(sources, source)
		}
	}
	return sources
}
