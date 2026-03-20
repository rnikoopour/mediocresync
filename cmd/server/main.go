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

	"github.com/rnikoopour/mediocresync/internal/api"
	"github.com/rnikoopour/mediocresync/internal/config"
	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/scheduler"
	internalsync "github.com/rnikoopour/mediocresync/internal/sync"
	"github.com/rnikoopour/mediocresync/internal/sse"
	"github.com/rnikoopour/mediocresync/ui"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	encKey, err := db.GetOrCreateEncryptionKey(database)
	if err != nil {
		slog.Error("load encryption key", "err", err)
		os.Exit(1)
	}

	auth := db.NewAuthRepository(database)
	connections := db.NewConnectionRepository(database)
	jobs := db.NewJobRepository(database)
	runs := db.NewRunRepository(database)
	transfers := db.NewTransferRepository(database)
	fileState := db.NewFileStateRepository(database)

	// Mark any runs left in "running" state from a previous unclean shutdown.
	if err := runs.CancelStaleRuns(); err != nil {
		slog.Error("cancel stale runs", "err", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := sse.NewBroker()

	engine := internalsync.NewEngine(
		connections,
		jobs,
		runs,
		transfers,
		fileState,
		encKey,
		broker,
		ctx,
	)

	sched := scheduler.NewScheduler(jobs, runs, engine)

	router := api.NewRouter(
		ctx,
		auth,
		connections,
		jobs,
		runs,
		transfers,
		fileState,
		engine,
		broker,
		encKey,
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
	cancel() // cancels in-flight runs and stops scheduler

	// Wait for any active runs to finish writing their final status.
	engine.Wait()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown", "err", err)
	}
	slog.Info("shutdown complete")
}
