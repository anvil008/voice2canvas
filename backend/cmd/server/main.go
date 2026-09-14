package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voice2canvas/backend/internal/server"
)

func main() {
	setupLogging()
	cfg := server.ConfigFromEnv()
	handler, err := server.NewHandler(cfg)
	if err != nil {
		slog.Error("build HTTP handler", "error", err)
		os.Exit(1)
	}
	address, err := server.ValidateListenAddr(cfg.ListenAddr)
	if err != nil {
		slog.Error("validate listen address", "address", cfg.ListenAddr, "error", err)
		os.Exit(1)
	}
	slog.Info("voice2canvas backend listening", "address", address)
	slog.Info("models", "live", cfg.LiveModel, "card", cfg.CardModel)

	srv := &http.Server{
		Addr:    address,
		Handler: handler,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			slog.Error("serve HTTP", "address", address, "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		slog.Info("server stopped gracefully")
	}
}
