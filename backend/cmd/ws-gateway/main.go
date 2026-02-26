package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moltgame/backend/internal/auth"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/moltgame/backend/internal/ws"
	"github.com/moltgame/backend/pkg/config"
	"github.com/moltgame/backend/pkg/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	ctx := context.Background()

	// Connect to PostgreSQL (for auth lookups)
	db, err := database.NewPostgresPool(ctx, cfg.DatabaseURL())
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

	// Initialize components
	agentRepo := auth.NewAgentRepository(db)
	hub := ws.NewHub()
	server := ws.NewServer(hub, nc, agentRepo)

	// Subscribe to NATS events for broadcasting to WebSocket clients
	if err := server.SubscribeNATSEvents(ctx); err != nil {
		slog.Error("failed to subscribe nats events", "error", err)
		os.Exit(1)
	}

	// HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"ws-gateway"}`))
	})
	mux.HandleFunc("GET /ws/game/{gameID}", server.HandleAgent)
	mux.HandleFunc("GET /ws/spectate/{gameID}", server.HandleSpectator)

	srv := &http.Server{
		Addr:         ":" + cfg.WSPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("ws-gateway starting", "port", cfg.WSPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down ws-gateway")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
