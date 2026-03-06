package api

import (
	_ "embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/moltgame/backend/internal/aibot"
	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/internal/chakra"
	gameRepo "github.com/moltgame/backend/internal/game"
	"github.com/moltgame/backend/internal/matchmaking"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/moltgame/backend/internal/twitter"
	"github.com/moltgame/backend/pkg/httputil"
)

type RouterDeps struct {
	AgentRepo     *auth.AgentRepository
	ChakraRepo    *chakra.Repository
	GameRepo      *gameRepo.Repository
	NATS          *natsClient.Client
	Settlement    *gameRepo.SettlementService
	MatchSvc      *matchmaking.Service
	TwitterClient *twitter.Client
	Sessions      *auth.SessionManager
	AIRunner      *aibot.Runner
	AdminPassword string
	SkipClaim     bool // dev mode: auto-activate agents on registration
}

//go:embed skill.md
var skillMD []byte

func serveSkillMD(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(skillMD)
}

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Global middleware (no Timeout here — applied per-route group to support long-polling)
	r.Use(middleware.RequestID)
	r.Use(slogMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://moltgame.com", "https://game.0ai.ai"},
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

	// Serve skill.md for AI agent discovery (embedded at build time)
	r.Get("/skill.md", serveSkillMD)
	r.Get("/.well-known/skill.md", serveSkillMD)

	// Handlers
	agentHandler := NewAgentHandler(deps.AgentRepo, deps.ChakraRepo, deps.SkipClaim)
	ownerHandler := NewOwnerHandler(deps.AgentRepo, deps.ChakraRepo, deps.TwitterClient)
	gameProxy := NewGameProxyHandler(deps.NATS, deps.GameRepo, deps.AgentRepo, deps.Settlement)
	matchHandler := NewMatchmakingHandler(deps.MatchSvc, deps.AgentRepo)
	authHandler := NewAuthHandler(deps.TwitterClient, deps.Sessions)
	adminHandler := NewAdminHandler(deps.AIRunner, deps.AdminPassword)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Agent long-polling — NO standard timeout (uses its own 65s)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAgent(deps.AgentRepo))
			r.Use(auth.RequireActiveAgent(deps.AgentRepo))
			r.Use(middleware.Timeout(65 * time.Second))
			r.Get("/agent/wait", gameProxy.AgentWait)
		})

		// All other routes use standard 30s timeout
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(30 * time.Second))

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

			// Game routes (proxied to poker-engine via NATS)
			r.Get("/games/live", gameProxy.ListLiveGames)
			r.Get("/games/recent", gameProxy.ListRecentGames)

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAgent(deps.AgentRepo))
				r.Use(auth.RequireActiveAgent(deps.AgentRepo))
				r.Post("/games", gameProxy.CreateGame)
				r.Get("/games/{id}/state", gameProxy.GetGameState)
				r.Post("/games/{id}/action", gameProxy.SubmitAction)
			})

			r.Get("/games/{id}/spectate", gameProxy.GetSpectatorState)
			r.Get("/games/{id}/events", gameProxy.GetGameHistory)

			// Matchmaking routes
			r.Get("/matchmaking/status", matchHandler.QueueStatus)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAgent(deps.AgentRepo))
				r.Use(auth.RequireActiveAgent(deps.AgentRepo))
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

			// Admin routes (password-protected in handler)
			r.Post("/admin/start-ai-game", adminHandler.StartAIGame)
			r.Get("/admin/ai-game-status", adminHandler.GetAIGameStatus)
		})
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
