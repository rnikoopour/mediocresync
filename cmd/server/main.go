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

	"github.com/rnikoopour/go-ftpes/internal/api"
	"github.com/rnikoopour/go-ftpes/internal/config"
	"github.com/rnikoopour/go-ftpes/internal/db"
	"github.com/rnikoopour/go-ftpes/internal/scheduler"
	internalsync "github.com/rnikoopour/go-ftpes/internal/sync"
	"github.com/rnikoopour/go-ftpes/internal/sse"
	"github.com/rnikoopour/go-ftpes/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	connections := db.NewConnectionRepository(database)
	jobs := db.NewJobRepository(database)
	runs := db.NewRunRepository(database)
	transfers := db.NewTransferRepository(database)
	fileState := db.NewFileStateRepository(database)

	broker := sse.NewBroker()

	engine := internalsync.NewEngine(
		connections,
		jobs,
		runs,
		transfers,
		fileState,
		cfg.EncryptionKey,
		broker,
	)

	sched := scheduler.NewScheduler(jobs, runs, engine)

	router := api.NewRouter(
		connections,
		jobs,
		runs,
		transfers,
		fileState,
		engine,
		broker,
		cfg.EncryptionKey,
		cfg.DevMode,
		ui.FS(),
	)

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE streams are long-lived; no write timeout
		IdleTimeout:  120 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	slog.Info("scheduler started")

	go func() {
		slog.Info("server listening", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	cancel() // stop scheduler from firing new runs

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown", "err", err)
	}
	slog.Info("shutdown complete")
}
