package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moltgame/backend/internal/api"
	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/internal/chakra"
	gameRepo "github.com/moltgame/backend/internal/game"
	"github.com/moltgame/backend/internal/matchmaking"
	"github.com/moltgame/backend/internal/models"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/moltgame/backend/internal/twitter"
	"github.com/moltgame/backend/pkg/config"
	"github.com/moltgame/backend/pkg/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	ctx := context.Background()

	// Connect to PostgreSQL
	db, err := database.NewPostgresPool(ctx, cfg.DatabaseURL())
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Connect to Redis
	rdb, err := database.NewRedisClient(ctx, cfg.RedisAddr)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// Connect to NATS
	nc, err := natsClient.Connect(cfg.NATSAddr)
	if err != nil {
		slog.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	// Initialize repositories
	agentRepo := auth.NewAgentRepository(db)
	chakraRepo := chakra.NewRepository(db)
	gameRepository := gameRepo.NewRepository(db)

	// Initialize services
	settlement := gameRepo.NewSettlementService(gameRepository, chakraRepo)
	sessions := auth.NewSessionManager(cfg.JWTSecret)

	// Initialize Twitter client
	twitterClient := twitter.NewClient(
		cfg.TwitterClientID,
		cfg.TwitterClientSecret,
		cfg.TwitterCallbackURL,
		cfg.TwitterAPIKey,
		cfg.TwitterAPIKeySecret,
		cfg.TwitterAccessToken,
		cfg.TwitterAccessTokenSecret,
	)

	// Build game proxy and subscribe to game-over events
	gameProxy := api.NewGameProxyHandler(nc, gameRepository, settlement)
	if err := gameProxy.SubscribeGameOver(ctx); err != nil {
		slog.Error("failed to subscribe game over events", "error", err)
		os.Exit(1)
	}

	// Matchmaking callback: when a match is formed, create the game via NATS
	matchSvc := matchmaking.NewService(nc, func(ctx context.Context, gameType models.GameType, players []*matchmaking.QueueEntry) error {
		if gameType != models.GameTypePoker {
			slog.Warn("unsupported game type for matchmaking", "type", gameType)
			return nil
		}

		playerIDs := make([]string, len(players))
		for i, p := range players {
			playerIDs[i] = p.AgentID
		}

		// Create DB record
		config := []byte(`{}`)
		dbGame, err := gameRepository.CreateGame(ctx, gameType, playerIDs, config)
		if err != nil {
			return err
		}

		// Create room via NATS
		seed := cryptoSeed()
		var resp natsClient.CreateRoomResponse
		err = nc.RequestJSON(natsClient.SubjectPokerRoomCreate, natsClient.CreateRoomRequest{
			GameID:    dbGame.ID,
			PlayerIDs: playerIDs,
			Seed:      seed,
			EntryFee:  matchmaking.DefaultConfigs[models.GameTypePoker].EntryFee,
		}, &resp, 3*time.Second)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("poker engine: %s", resp.Error)
		}

		return nil
	})

	// Start matchmaking loop
	matchCtx, matchCancel := context.WithCancel(ctx)
	defer matchCancel()
	go matchSvc.RunMatchLoop(matchCtx, 5*time.Second)

	// Start passive Chakra regen scheduler (every hour)
	regenCtx, regenCancel := context.WithCancel(ctx)
	defer regenCancel()
	go chakra.RunPassiveRegenLoop(regenCtx, chakraRepo, 1*time.Hour)

	// Build router
	router := api.NewRouter(api.RouterDeps{
		AgentRepo:     agentRepo,
		ChakraRepo:    chakraRepo,
		GameRepo:      gameRepository,
		NATS:          nc,
		Settlement:    settlement,
		MatchSvc:      matchSvc,
		TwitterClient: twitterClient,
		Sessions:      sessions,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 70 * time.Second, // supports 60s long-polling + buffer
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("api-gateway starting", "port", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down api-gateway")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

func cryptoSeed() int64 {
	var b [8]byte
	rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}
