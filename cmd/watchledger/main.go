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

	"github.com/zekihan/watchledger/internal/anidb"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/httpapi"
	"github.com/zekihan/watchledger/internal/jobs"
	"github.com/zekihan/watchledger/internal/store"
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
	c, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := store.Open(ctx, c.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	if err = store.Migrate(ctx, pool); err != nil {
		return err
	}
	identity, err := auth.New(ctx, pool, c)
	if err != nil {
		return err
	}
	runner := &jobs.Runner{Cache: &catalog.Cache{DB: pool, Provider: catalog.New(c.TMDBToken)}, Anime: &anidb.Cache{DB: pool, Provider: anidb.New(anidb.Config{Client: c.AniDBClient, Version: c.AniDBVersion})}}
	jobCtx, cancelJobs := context.WithCancel(ctx)
	jobsDone := make(chan struct{})
	go func() { defer close(jobsDone); runner.Run(jobCtx) }()
	defer func() { cancelJobs(); <-jobsDone }()
	handler := httpapi.NewApplication(c.WebDir, pool, identity, c.TMDBToken, anidb.Config{Client: c.AniDBClient, Version: c.AniDBVersion})

	server := &http.Server{
		Addr:              c.Address,
		Handler:           httpapi.RequestBudget(handler),
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      6 * time.Minute,
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

	slog.Info("starting watchledger", "address", c.Address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
