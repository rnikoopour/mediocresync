package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rnikoopour/mediocresync/internal/api"
	"github.com/rnikoopour/mediocresync/internal/config"
	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/logbuffer"
	"github.com/rnikoopour/mediocresync/internal/scheduler"
	internalsync "github.com/rnikoopour/mediocresync/internal/sync"
	"github.com/rnikoopour/mediocresync/internal/sse"
	"github.com/rnikoopour/mediocresync/ui"
)

func initLogger(level config.LogLevel) (*slog.LevelVar, *logbuffer.Buffer) {
	var lv slog.LevelVar
	switch level {
	case config.LogLevelDebug:
		lv.Set(slog.LevelDebug)
	case config.LogLevelWarn:
		lv.Set(slog.LevelWarn)
	case config.LogLevelError:
		lv.Set(slog.LevelError)
	default:
		lv.Set(slog.LevelInfo)
	}
	opts := &slog.HandlerOptions{Level: &lv}
	buf := logbuffer.New(logbuffer.DefaultSize, slog.NewTextHandler(os.Stderr, opts))
	slog.SetDefault(slog.New(buf))
	return &lv, buf
}

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	cfg := config.Load()
	logLevel, logBuf := initLogger(cfg.LogLevel)

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
	sources := db.NewSourceRepository(database)
	gitRepos := db.NewGitRepoRepository(database)
	jobs := db.NewJobRepository(database)
	runs := db.NewRunRepository(database)
	transfers := db.NewTransferRepository(database)
	syncState := db.NewSyncStateRepository(database)

	// Mark any runs left in "running" state from a previous unclean shutdown.
	if err := runs.CancelStaleRuns(); err != nil {
		slog.Error("cancel stale runs", "err", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := sse.NewBroker()

	engine := internalsync.NewEngine(
		sources,
		gitRepos,
		jobs,
		runs,
		transfers,
		syncState,
		encKey,
		broker,
		ctx,
	)

	sched := scheduler.NewScheduler(jobs, runs, engine, broker)

	router := api.NewRouter(
		ctx,
		version,
		auth,
		sources,
		jobs,
		runs,
		transfers,
		syncState,
		engine,
		broker,
		encKey,
		cfg.DevMode,
		logLevel,
		logBuf,
		ui.FS(),
	)

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE streams are long-lived; no write timeout
		IdleTimeout:  120 * time.Second,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
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
