package api

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moltgame/backend/internal/auth"
	gameRepo "github.com/moltgame/backend/internal/game"
	"github.com/moltgame/backend/internal/models"
	"github.com/moltgame/backend/internal/room"
	"github.com/moltgame/backend/pkg/httputil"
)

// GameHandler handles game-related HTTP requests.
type GameHandler struct {
	rooms      *room.Manager
	gameRepo   *gameRepo.Repository
	settlement *gameRepo.SettlementService
}

// NewGameHandler creates a new game handler.
func NewGameHandler(rooms *room.Manager, repo *gameRepo.Repository, settlement *gameRepo.SettlementService) *GameHandler {
	return &GameHandler{
		rooms:      rooms,
		gameRepo:   repo,
		settlement: settlement,
	}
}

// --- Request/Response types ---

type createGameRequest struct {
	Type      models.GameType `json:"type"`
	PlayerIDs []string        `json:"player_ids"`
	EntryFee  int             `json:"entry_fee"`
}

type createGameResponse struct {
	GameID    string          `json:"game_id"`
	Type      models.GameType `json:"game_type"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
}

type submitActionRequest struct {
	Action json.RawMessage `json:"action"`
}

// --- Handlers ---

// CreateGame creates a new game room and starts it.
// POST /api/v1/games
func (h *GameHandler) CreateGame(w http.ResponseWriter, r *http.Request) {
	var req createGameRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	// Validate game type
	switch req.Type {
	case models.GameTypePoker, models.GameTypeWerewolf:
	default:
		httputil.Error(w, http.StatusBadRequest, "invalid_type", "Game type must be 'poker' or 'werewolf'")
		return
	}

	// Validate player count
	if len(req.PlayerIDs) < 2 {
		httputil.Error(w, http.StatusBadRequest, "invalid_players", "Need at least 2 players")
		return
	}

	// Create DB record
	config, _ := json.Marshal(map[string]interface{}{
		"entry_fee": req.EntryFee,
	})
	dbGame, err := h.gameRepo.CreateGame(r.Context(), req.Type, req.PlayerIDs, config)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "create_failed", "Failed to create game")
		return
	}

	// Collect entry fees
	if req.EntryFee > 0 {
		if err := h.settlement.CollectEntryFees(r.Context(), dbGame.ID, req.PlayerIDs, req.EntryFee); err != nil {
			httputil.Error(w, http.StatusPaymentRequired, "insufficient_chakra", err.Error())
			return
		}
	}

	// Create in-memory game room
	seed := cryptoSeed()
	switch req.Type {
	case models.GameTypePoker:
		_, err = h.rooms.CreatePokerRoom(dbGame.ID, req.PlayerIDs, seed)
	case models.GameTypeWerewolf:
		_, _, err = h.rooms.CreateWerewolfRoom(dbGame.ID, req.PlayerIDs, seed)
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "room_failed", err.Error())
		return
	}

	httputil.JSON(w, http.StatusCreated, createGameResponse{
		GameID:    dbGame.ID,
		Type:      req.Type,
		Status:    "playing",
		CreatedAt: dbGame.CreatedAt,
	})
}

// GetGameState returns the game state for the authenticated agent.
// GET /api/v1/games/{id}/state
func (h *GameHandler) GetGameState(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")
	agentID := auth.GetAgentID(r.Context())

	rm := h.rooms.GetRoom(gameID)
	if rm == nil {
		httputil.Error(w, http.StatusNotFound, "game_not_found", "Game not found or already finished")
		return
	}

	if !rm.HasPlayer(agentID) {
		httputil.Error(w, http.StatusForbidden, "not_in_game", "You are not a player in this game")
		return
	}

	state, err := rm.GetState(agentID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}

	httputil.JSON(w, http.StatusOK, state)
}

// SubmitAction processes a player action.
// POST /api/v1/games/{id}/action
func (h *GameHandler) SubmitAction(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")
	agentID := auth.GetAgentID(r.Context())

	rm := h.rooms.GetRoom(gameID)
	if rm == nil {
		httputil.Error(w, http.StatusNotFound, "game_not_found", "Game not found or already finished")
		return
	}

	if !rm.HasPlayer(agentID) {
		httputil.Error(w, http.StatusForbidden, "not_in_game", "You are not a player in this game")
		return
	}

	var req submitActionRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	result, err := rm.SubmitAction(agentID, req.Action)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "action_error", err.Error())
		return
	}

	// If game is over, trigger settlement asynchronously
	if result.GameOver {
		go h.SettleGame(gameID, rm)
	}

	httputil.JSON(w, http.StatusOK, result)
}

// GetSpectatorState returns the god-view state for spectators.
// GET /api/v1/games/{id}/spectate
func (h *GameHandler) GetSpectatorState(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")

	rm := h.rooms.GetRoom(gameID)
	if rm == nil {
		httputil.Error(w, http.StatusNotFound, "game_not_found", "Game not found or already finished")
		return
	}

	state, err := rm.GetSpectatorState()
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}

	httputil.JSON(w, http.StatusOK, state)
}

// ListLiveGames returns currently active games.
// GET /api/v1/games/live
func (h *GameHandler) ListLiveGames(w http.ResponseWriter, r *http.Request) {
	games := h.rooms.ListActiveGames()
	httputil.JSON(w, http.StatusOK, games)
}

// GetGameHistory retrieves a finished game's events for replay.
// GET /api/v1/games/{id}/events
func (h *GameHandler) GetGameHistory(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")

	events, err := h.gameRepo.GetGameEvents(r.Context(), gameID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "game_not_found", "Game not found")
		return
	}

	httputil.JSON(w, http.StatusOK, events)
}

// ListRecentGames returns recently finished games.
// GET /api/v1/games/recent
func (h *GameHandler) ListRecentGames(w http.ResponseWriter, r *http.Request) {
	games, err := h.gameRepo.ListRecentGames(r.Context(), 20)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "query_failed", "Failed to list recent games")
		return
	}
	if games == nil {
		games = []gameRepo.RecentGame{}
	}
	httputil.JSON(w, http.StatusOK, games)
}

// --- Internal helpers ---

// SettleGame processes end-of-game settlement (events persistence + Chakra + TrueSkill).
// It is called both from the API SubmitAction handler and from the Room's OnGameOver callback (timeouts).
func (h *GameHandler) SettleGame(gameID string, rm *room.Room) {
	defer h.rooms.RemoveRoom(gameID)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Persist accumulated events (Event Sourcing)
	events := rm.GetAccumulatedEvents()
	if len(events) > 0 {
		if err := h.gameRepo.RecordEvents(ctx, gameID, 1, events); err != nil {
			slog.Error("failed to persist game events", "game_id", gameID, "error", err)
			// Continue with settlement even if event persistence fails
		} else {
			slog.Info("persisted game events", "game_id", gameID, "count", len(events))
		}
	}

	// 2. Build rankings and settle
	switch rm.GameType {
	case models.GameTypePoker:
		if rm.Poker == nil || !rm.Poker.Finished {
			return
		}
		rankings := rm.Poker.GetRankings()
		results := make([]gameRepo.PlayerResult, len(rankings))
		var winnerID *string
		for i, r := range rankings {
			results[i] = gameRepo.PlayerResult{
				AgentID: r.PlayerID,
				Rank:    r.Rank,
			}
			if r.Rank == 1 {
				winnerID = &r.PlayerID
			}
		}
		if err := h.settlement.Settle(ctx, gameRepo.SettleConfig{
			GameID:   gameID,
			GameType: models.GameTypePoker,
			EntryFee: 20, // standard poker entry fee
			RakeRate: 0.10,
			Results:  results,
			WinnerID: winnerID,
		}); err != nil {
			slog.Error("poker settlement failed", "game_id", gameID, "error", err)
		} else {
			slog.Info("poker game settled", "game_id", gameID, "winner", winnerID)
		}

	case models.GameTypeWerewolf:
		if rm.Werewolf == nil || !rm.Werewolf.IsGameOver() {
			return
		}
		winningTeam := rm.Werewolf.WinningTeam()
		var results []gameRepo.PlayerResult
		var winnerID *string
		for _, p := range rm.Werewolf.Players {
			rank := 2
			if p.Team == winningTeam {
				rank = 1
				if winnerID == nil {
					id := p.ID
					winnerID = &id
				}
			}
			results = append(results, gameRepo.PlayerResult{
				AgentID: p.ID,
				Rank:    rank,
			})
		}
		if err := h.settlement.Settle(ctx, gameRepo.SettleConfig{
			GameID:   gameID,
			GameType: models.GameTypeWerewolf,
			EntryFee: 30, // standard werewolf entry fee
			RakeRate: 0.10,
			Results:  results,
			WinnerID: winnerID,
		}); err != nil {
			slog.Error("werewolf settlement failed", "game_id", gameID, "error", err)
		} else {
			slog.Info("werewolf game settled", "game_id", gameID, "winner", winnerID)
		}
	}
}

func cryptoSeed() int64 {
	var b [8]byte
	rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}
