package main

import (
	"log/slog"
	"net/http"
	"os"

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
	if err := http.ListenAndServe(address, handler); err != nil {
		slog.Error("serve HTTP", "address", address, "error", err)
		os.Exit(1)
	}
}
