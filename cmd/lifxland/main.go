// Command lifxland exposes a long-lived LIFX LAN controller over HTTP.
package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/control"
	"github.com/alessio-palumbo/lifxlan-go/internal/httpapi"
	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
)

const shutdownTimeout = 5 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("lifxland stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	listenAddress := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	apiToken := flag.String("api-token", os.Getenv("LIFXLAN_API_TOKEN"), "bearer token (or LIFXLAN_API_TOKEN)")
	flag.Parse()

	if err := validateExposure(*listenAddress, *apiToken); err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctrl, err := controller.New(controller.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}
	defer ctrl.Close()

	handler := httpapi.NewHandler(control.New(ctrl))
	if *apiToken != "" {
		handler = bearerAuth(handler, *apiToken)
	}
	server := &http.Server{
		Addr:              *listenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP API listening", "address", *listenAddress)
		serverErr <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP API: %w", err)
		}
		return nil
	case sig := <-signals:
		logger.Info("shutting down", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shut down HTTP API: %w", err)
	}
	return nil
}

func validateExposure(address, token string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if isLoopbackHost(host) || token != "" {
		return nil
	}
	return errors.New("a bearer token is required when listening beyond loopback")
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func bearerAuth(next http.Handler, token string) http.Handler {
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		provided := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(provided, expected) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("{\"error\":{\"code\":\"unauthorized\",\"message\":\"a valid bearer token is required\"}}\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
