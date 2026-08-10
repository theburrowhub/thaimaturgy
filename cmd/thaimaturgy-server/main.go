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
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/appservice"
	"github.com/theburrowhub/thaimaturgy/internal/auth"
	"github.com/theburrowhub/thaimaturgy/internal/httpapi"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
	"github.com/theburrowhub/thaimaturgy/internal/storage"
)

func main() {
	addr := flag.String("addr", envOr("THAIM_ADDR", "127.0.0.1:8765"), "listen address (host:port)")
	token := flag.String("token", os.Getenv("THAIM_SERVER_TOKEN"), "require this bearer token on /api/ (recommended when not on loopback)")
	flag.Parse()

	store, err := storage.New()
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
	svc := appservice.New(store, config, providers.New(config))

	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(svc, strings.TrimSpace(*token)).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	if !isLoopback(*addr) && strings.TrimSpace(*token) == "" {
		log.Printf("WARNING: binding %s beyond loopback WITHOUT a token — anyone who can reach it controls your games and data. Set THAIM_SERVER_TOKEN.", *addr)
	}
	log.Printf("thaimaturgy-server on http://%s — %s", *addr, msg)

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
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

// isLoopback reports whether a host:port binds only to loopback.
func isLoopback(addr string) bool {
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
	}
	host = strings.Trim(host, "[]")
	return host == "" || host == "127.0.0.1" || host == "::1" || host == "localhost"
}
