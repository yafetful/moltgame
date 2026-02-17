package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/internal/chakra"
	gameRepo "github.com/moltgame/backend/internal/game"
	"github.com/moltgame/backend/internal/matchmaking"
	"github.com/moltgame/backend/internal/room"
	"github.com/moltgame/backend/internal/twitter"
	"github.com/moltgame/backend/pkg/httputil"
)

type RouterDeps struct {
	AgentRepo     *auth.AgentRepository
	ChakraRepo    *chakra.Repository
	GameRepo      *gameRepo.Repository
	Rooms         *room.Manager
	Settlement    *gameRepo.SettlementService
	MatchSvc      *matchmaking.Service
	TwitterClient *twitter.Client
	Sessions      *auth.SessionManager
}

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(slogMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://moltgame.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "api-gateway"})
	})

	// Handlers
	agentHandler := NewAgentHandler(deps.AgentRepo)
	ownerHandler := NewOwnerHandler(deps.AgentRepo, deps.ChakraRepo, deps.TwitterClient)
	gameHandler := NewGameHandler(deps.Rooms, deps.GameRepo, deps.Settlement)
	// Wire up OnGameOver for timeout-triggered settlements
	deps.Rooms.OnGameOver = gameHandler.SettleGame
	matchHandler := NewMatchmakingHandler(deps.MatchSvc, deps.AgentRepo)
	authHandler := NewAuthHandler(deps.TwitterClient, deps.Sessions)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Post("/agents/register", agentHandler.Register)
		r.Get("/agents/{name}", agentHandler.GetByName)

		// Twitter OAuth
		r.Get("/auth/twitter", authHandler.StartTwitterAuth)
		r.Post("/auth/twitter/callback", authHandler.TwitterCallback)

		// Agent-authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAgent(deps.AgentRepo))
			r.Get("/agents/me", agentHandler.GetMe)
			r.Patch("/agents/me", agentHandler.UpdateMe)
			r.Get("/agents/me/status", agentHandler.GetStatus)
		})

		// Game routes
		r.Get("/games/live", gameHandler.ListLiveGames)
		r.Get("/games/recent", gameHandler.ListRecentGames)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAgent(deps.AgentRepo))
			r.Post("/games", gameHandler.CreateGame)
			r.Get("/games/{id}/state", gameHandler.GetGameState)
			r.Post("/games/{id}/action", gameHandler.SubmitAction)
		})

		r.Get("/games/{id}/spectate", gameHandler.GetSpectatorState)
		r.Get("/games/{id}/events", gameHandler.GetGameHistory)

		// Matchmaking routes
		r.Get("/matchmaking/status", matchHandler.QueueStatus)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAgent(deps.AgentRepo))
			r.Post("/matchmaking/join", matchHandler.JoinQueue)
			r.Delete("/matchmaking/leave", matchHandler.LeaveQueue)
		})

		// Owner routes (JWT session required)
		r.Route("/owner", func(r chi.Router) {
			r.Use(auth.RequireOwner(deps.Sessions))
			r.Get("/agents", ownerHandler.GetMyAgents)
			r.Post("/agents/{id}/rotate-key", ownerHandler.RotateKey)
			r.Post("/agents/{id}/check-in", ownerHandler.CheckIn)
		})

		// Agent claim (requires owner JWT — twitter_id from JWT, not request body)
		r.With(auth.RequireOwner(deps.Sessions)).Post("/agents/claim", ownerHandler.ClaimAgent)
	})

	return r
}

func slogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", ww.BytesWritten(),
		)
	})
}
