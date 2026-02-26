package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moltgame/backend/internal/engine"
	"github.com/moltgame/backend/internal/game"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/moltgame/backend/pkg/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	ctx := context.Background()

	// Connect to PostgreSQL for incremental event persistence
	db, err := pgxpool.New(ctx, cfg.DatabaseURL())
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Connect to NATS
	nc, err := natsClient.Connect(cfg.NATSAddr)
	if err != nil {
		slog.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	// Create and start poker engine
	eng := engine.NewPokerEngine(nc)
	eng.SetGameRepo(game.NewRepository(db))

	if err := eng.Start(ctx); err != nil {
		slog.Error("failed to start poker engine", "error", err)
		os.Exit(1)
	}

	slog.Info("poker-engine started", "nats", cfg.NATSAddr, "db", "connected")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down poker-engine")
	eng.Stop()
}
