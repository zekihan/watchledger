package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zekihan/watchledger/internal/httpapi"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	address := os.Getenv("WATCHLEDGER_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	server := &http.Server{
		Addr:              address,
		Handler:           httpapi.NewHandler(os.Getenv("WATCHLEDGER_WEB_DIR")),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				slog.Error("graceful shutdown failed", "error", err)
				if closeErr := server.Close(); closeErr != nil {
					slog.Error("server close failed", "error", closeErr)
				}
			}
		case <-done:
		}
	}()

	slog.Info("starting skeleton server", "address", address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
