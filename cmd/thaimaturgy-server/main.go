// Command thaimaturgy-server runs thAImaturgy as an HTTP server over the
// appservice facade (issue #36, Phase B): a JSON REST API plus an SSE stream for
// live session updates. It reuses the same storage/config/provider setup as the
// desktop app, so it operates on the same ~/.thaimaturgy data.
//
// It binds to loopback by default; expose it beyond localhost only with an auth
// token (THAIM_SERVER_TOKEN) and, ideally, behind a TLS-terminating proxy.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/httpapi"
	"github.com/theburrowhub/thaimaturgy/internal/mcpserve"
	"github.com/theburrowhub/thaimaturgy/internal/mcptools"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

// shutdownSignals are the signals that trigger a graceful shutdown. SIGTERM is
// included because Docker/Kubernetes/systemd stop services with it.
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func main() {
	if len(os.Args) > 1 && os.Args[1] == mcptools.SubcommandArg {
		if err := mcpserve.RunSubcommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "mcp-tools:", err)
			os.Exit(1)
		}
		return
	}
	addr := flag.String("addr", envOr("THAIM_ADDR", "127.0.0.1:8765"), "listen address (host:port)")
	token := flag.String("token", os.Getenv("THAIM_SERVER_TOKEN"), "require this bearer token on /api/ (recommended when not on loopback)")
	flag.Parse()

	// A data dir can be pinned so a container can mount one catalog volume;
	// otherwise the platform default is used.
	store, err := storage.NewFromEnvironment()
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	_ = store.LoadEnvFile()
	config, err := store.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	msg := auth.AutoConfigure(config)
	if config.RunModel != "" {
		config.Model = config.RunModel
	}
	if !store.ConfigExists() {
		_ = store.SaveConfig(config)
	}
	svc, err := appservice.New(store, config, providers.New(config))
	if err != nil {
		log.Fatalf("service: %v", err)
	}
	defer svc.Close()
	if diagnostics := svc.RulesDiagnostics(); diagnostics != nil {
		log.Printf("rules catalog diagnostics: %v", diagnostics)
	}

	tok := strings.TrimSpace(*token)
	loopback, err := isLoopbackAddr(*addr)
	if err != nil {
		log.Fatalf("invalid --addr %q: %v", *addr, err)
	}
	// Fail closed: never expose the API beyond loopback without a token — it would
	// hand session commands, deletion, config, and billed oracle calls to anyone.
	if !loopback && tok == "" {
		log.Fatalf("refusing to bind %q beyond loopback without a token: set THAIM_SERVER_TOKEN (or bind 127.0.0.1)", *addr)
	}

	// Rebuild the LLM provider when the config is saved over the API, so new
	// credentials/model take effect without a restart (mirrors the desktop app).
	h := httpapi.New(svc, tok).OnConfigSaved(func(cfg *domain.Config) {
		_ = auth.AutoConfigure(cfg)
		svc.SetProvider(providers.New(cfg))
	})
	srv := &http.Server{
		Addr:              *addr,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("thaimaturgy-server on http://%s — %s", *addr, msg)

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()
	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	log.Printf("thaimaturgy-server stopped")
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// isLoopbackAddr reports whether a host:port binds ONLY to loopback. An empty
// host (e.g. ":8765") is a wildcard bind to every interface, so it is NOT
// loopback. A non-IP hostname is treated conservatively as non-loopback.
func isLoopbackAddr(addr string) (bool, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false, err
	}
	if host == "" {
		return false, nil // wildcard → all interfaces
	}
	if host == "localhost" {
		return true, nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false, nil // a hostname we can't classify → assume exposed
	}
	return ip.IsLoopback(), nil
}
