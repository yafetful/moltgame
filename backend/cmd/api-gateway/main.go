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

	"github.com/moltgame/backend/internal/aibot"
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
	ownerRepo := auth.NewOwnerRepository(db)
	tokenStore := auth.NewOwnerTokenStore(rdb)
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
	gameProxy := api.NewGameProxyHandler(nc, gameRepository, agentRepo, settlement)
	if err := gameProxy.SubscribeGameOver(ctx); err != nil {
		slog.Error("failed to subscribe game over events", "error", err)
		os.Exit(1)
	}

	// Initialize AI bot runner (optional — only if OPENROUTER_API_KEY is set)
	var aiRunner *aibot.Runner
	if cfg.OpenRouterAPIKey != "" && cfg.AIModel != "" {
		agentNames := []string{"molt-ace", "molt-bluff", "molt-chip", "molt-deal", "molt-edge", "molt-fold"}
		aiAgents := make([]aibot.AgentConfig, 6)
		for i, name := range agentNames {
			aiAgents[i] = aibot.AgentConfig{Name: name, Model: cfg.AIModel}
		}
		aiRunner = aibot.NewRunner(nc, agentRepo, gameRepository, settlement, cfg.OpenRouterAPIKey, aiAgents)
		slog.Info("AI bot runner initialized", "model", cfg.AIModel)
	}

	// Matchmaking callback: when a match is formed, create the game via NATS
	var matchSvc *matchmaking.Service
	matchSvc = matchmaking.NewService(nc, func(ctx context.Context, gameType models.GameType, players []*matchmaking.QueueEntry) error {
		if gameType != models.GameTypePoker {
			slog.Warn("unsupported game type for matchmaking", "type", gameType)
			return nil
		}

		playerIDs := make([]string, len(players))
		for i, p := range players {
			playerIDs[i] = p.AgentID
		}

		entryFee := matchmaking.DefaultConfigs[models.GameTypePoker].EntryFee

		// Create DB record
		cfgJSON := []byte(fmt.Sprintf(`{"entry_fee":%d}`, entryFee))
		dbGame, err := gameRepository.CreateGame(ctx, gameType, playerIDs, cfgJSON)
		if err != nil {
			return err
		}

		// Collect entry fees (skip house bots)
		if entryFee > 0 {
			var realPlayerIDs []string
			for _, id := range playerIDs {
				if aiRunner != nil && aiRunner.IsBotAgent(ctx, id) {
					continue
				}
				realPlayerIDs = append(realPlayerIDs, id)
			}
			if len(realPlayerIDs) > 0 {
				if err := settlement.CollectEntryFees(ctx, dbGame.ID, realPlayerIDs, entryFee); err != nil {
					return fmt.Errorf("entry fee: %w", err)
				}
			}
		}

		// Look up agent names and avatars for display
		playerNames := make(map[string]string)
		playerAvatars := make(map[string]string)
		for _, p := range players {
			if agent, err := agentRepo.GetAgentByID(ctx, p.AgentID); err == nil {
				playerNames[p.AgentID] = agent.Name
				playerAvatars[p.AgentID] = agent.AvatarURL
			}
		}

		// Create room via NATS
		seed := cryptoSeed()
		var resp natsClient.CreateRoomResponse
		err = nc.RequestJSON(natsClient.SubjectPokerRoomCreate, natsClient.CreateRoomRequest{
			GameID:        dbGame.ID,
			PlayerIDs:     playerIDs,
			PlayerNames:   playerNames,
			PlayerAvatars: playerAvatars,
			Seed:          seed,
			EntryFee:      entryFee,
		}, &resp, 3*time.Second)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("poker engine: %s", resp.Error)
		}

		// Publish match_found notification
		matchSvc.PublishMatchFound(dbGame.ID, gameType, playerIDs)

		// Start AI bot driver for any house bots in this game
		if aiRunner != nil {
			var botIDs []string
			for _, id := range playerIDs {
				if aiRunner.IsBotAgent(ctx, id) {
					botIDs = append(botIDs, id)
				}
			}
			if len(botIDs) > 0 {
				aiRunner.RunBotsForGame(dbGame.ID, botIDs, playerNames)
			}
		}

		return nil
	})

	// Wire up AI bot backfill provider and busy-bot checker
	if aiRunner != nil {
		matchSvc.SetBotProvider(aiRunner)
		matchSvc.SetBusyBotChecker(gameRepository)
	}

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
		OwnerRepo:     ownerRepo,
		TokenStore:    tokenStore,
		AIRunner:      aiRunner,
		AdminPassword: cfg.AdminPassword,
		SkipClaim:     cfg.SkipClaim,
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
