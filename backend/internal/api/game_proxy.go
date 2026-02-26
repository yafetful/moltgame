package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moltgame/backend/internal/auth"
	gameRepo "github.com/moltgame/backend/internal/game"
	"github.com/moltgame/backend/internal/models"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/moltgame/backend/pkg/httputil"
	"github.com/nats-io/nats.go"
)

const natsTimeout = 3 * time.Second

// GameProxyHandler proxies game requests to the poker-engine via NATS.
type GameProxyHandler struct {
	nats       *natsClient.Client
	gameRepo   *gameRepo.Repository
	settlement *gameRepo.SettlementService
}

// NewGameProxyHandler creates a new game proxy handler.
func NewGameProxyHandler(nc *natsClient.Client, repo *gameRepo.Repository, settlement *gameRepo.SettlementService) *GameProxyHandler {
	return &GameProxyHandler{
		nats:       nc,
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

// CreateGame creates a new game: DB record → NATS create room.
// POST /api/v1/games
func (h *GameProxyHandler) CreateGame(w http.ResponseWriter, r *http.Request) {
	var req createGameRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Type != models.GameTypePoker {
		httputil.Error(w, http.StatusBadRequest, "invalid_type", "Only 'poker' is supported")
		return
	}

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

	// Create room via NATS
	seed := cryptoSeed()
	var resp natsClient.CreateRoomResponse
	err = h.nats.RequestJSON(natsClient.SubjectPokerRoomCreate, natsClient.CreateRoomRequest{
		GameID:    dbGame.ID,
		PlayerIDs: req.PlayerIDs,
		Seed:      seed,
		EntryFee:  req.EntryFee,
	}, &resp, natsTimeout)
	if err != nil {
		httputil.Error(w, http.StatusServiceUnavailable, "engine_unavailable", "Poker engine unavailable")
		return
	}
	if !resp.Success {
		httputil.Error(w, http.StatusInternalServerError, "room_failed", resp.Error)
		return
	}

	httputil.JSON(w, http.StatusCreated, createGameResponse{
		GameID:    dbGame.ID,
		Type:      req.Type,
		Status:    "playing",
		CreatedAt: dbGame.CreatedAt,
	})
}

// SubmitAction proxies action to poker-engine via NATS.
// POST /api/v1/games/{id}/action
func (h *GameProxyHandler) SubmitAction(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")
	agentID := auth.GetAgentID(r.Context())

	var req submitActionRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	var resp natsClient.ActionResponse
	err := h.nats.RequestJSON(natsClient.SubjectPokerRoomAction(gameID), natsClient.ActionRequest{
		AgentID: agentID,
		Action:  req.Action,
	}, &resp, natsTimeout)
	if err != nil {
		httputil.Error(w, http.StatusServiceUnavailable, "engine_unavailable", "Poker engine unavailable")
		return
	}
	if !resp.Success {
		httputil.Error(w, http.StatusBadRequest, "action_error", resp.Error)
		return
	}

	httputil.JSON(w, http.StatusOK, resp)
}

// GetGameState proxies state request to poker-engine.
// GET /api/v1/games/{id}/state
func (h *GameProxyHandler) GetGameState(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")
	agentID := auth.GetAgentID(r.Context())

	var resp natsClient.StateResponse
	err := h.nats.RequestJSON(natsClient.SubjectPokerRoomState(gameID), natsClient.StateRequest{
		AgentID: agentID,
	}, &resp, natsTimeout)
	if err != nil {
		httputil.Error(w, http.StatusServiceUnavailable, "engine_unavailable", "Poker engine unavailable")
		return
	}
	if !resp.Success {
		httputil.Error(w, http.StatusNotFound, "game_not_found", resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp.State)
}

// GetSpectatorState proxies spectator state request.
// GET /api/v1/games/{id}/spectate
func (h *GameProxyHandler) GetSpectatorState(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")

	var resp natsClient.StateResponse
	err := h.nats.RequestJSON(natsClient.SubjectPokerRoomSpectate(gameID), struct{}{}, &resp, natsTimeout)
	if err != nil {
		httputil.Error(w, http.StatusServiceUnavailable, "engine_unavailable", "Poker engine unavailable")
		return
	}
	if !resp.Success {
		httputil.Error(w, http.StatusNotFound, "game_not_found", resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp.State)
}

// ListLiveGames proxies live games list request.
// GET /api/v1/games/live
func (h *GameProxyHandler) ListLiveGames(w http.ResponseWriter, r *http.Request) {
	var resp natsClient.ListRoomsResponse
	err := h.nats.RequestJSON(natsClient.SubjectPokerRoomList, struct{}{}, &resp, natsTimeout)
	if err != nil {
		httputil.Error(w, http.StatusServiceUnavailable, "engine_unavailable", "Poker engine unavailable")
		return
	}

	httputil.JSON(w, http.StatusOK, resp.Games)
}

// GetGameHistory retrieves a finished game's events for replay (direct DB query).
// GET /api/v1/games/{id}/events
func (h *GameProxyHandler) GetGameHistory(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")

	events, err := h.gameRepo.GetGameEvents(r.Context(), gameID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "game_not_found", "Game not found")
		return
	}

	httputil.JSON(w, http.StatusOK, events)
}

// ListRecentGames returns recently finished games (direct DB query).
// GET /api/v1/games/recent
func (h *GameProxyHandler) ListRecentGames(w http.ResponseWriter, r *http.Request) {
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

// SubscribeGameOver subscribes to poker.gameover.* and triggers settlement.
func (h *GameProxyHandler) SubscribeGameOver(ctx context.Context) error {
	_, err := h.nats.Subscribe("poker.gameover.*", func(msg *nats.Msg) {
		var evt natsClient.GameOverEvent
		if err := json.Unmarshal(msg.Data, &evt); err != nil {
			slog.Error("failed to unmarshal game over event", "error", err)
			return
		}

		go h.settleGame(evt)
	})
	return err
}

func (h *GameProxyHandler) settleGame(evt natsClient.GameOverEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Persist any remaining events (fallback — most events are already
	//    persisted incrementally by the poker engine)
	var events []models.GameEvent
	if err := json.Unmarshal(evt.AccumulatedEvents, &events); err == nil && len(events) > 0 {
		// Check how many events are already in DB to avoid duplicates
		existing, _ := h.gameRepo.GetGameEvents(ctx, evt.GameID)
		if len(existing) < len(events) {
			remaining := events[len(existing):]
			if len(remaining) > 0 {
				if err := h.gameRepo.RecordEvents(ctx, evt.GameID, len(existing)+1, remaining); err != nil {
					slog.Error("failed to persist remaining events", "game_id", evt.GameID, "error", err)
				} else {
					slog.Info("persisted remaining events", "game_id", evt.GameID, "count", len(remaining))
				}
			}
		}
	}

	// 2. Parse rankings and settle
	type rankEntry struct {
		Rank     int    `json:"rank"`
		Seat     int    `json:"seat"`
		PlayerID string `json:"player_id"`
	}
	var rankings []rankEntry
	if err := json.Unmarshal(evt.Rankings, &rankings); err != nil {
		slog.Error("failed to unmarshal rankings", "game_id", evt.GameID, "error", err)
		return
	}

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
		GameID:   evt.GameID,
		GameType: models.GameTypePoker,
		EntryFee: evt.EntryFee,
		RakeRate: 0.10,
		Results:  results,
		WinnerID: winnerID,
	}); err != nil {
		slog.Error("poker settlement failed", "game_id", evt.GameID, "error", err)
	} else {
		slog.Info("poker game settled", "game_id", evt.GameID, "winner", winnerID)
	}

	// 3. Cleanup room in poker-engine
	h.nats.RequestJSON(natsClient.SubjectPokerRoomCleanup(evt.GameID), struct{}{}, &natsClient.CreateRoomResponse{}, natsTimeout)
}

// AgentWait implements long-polling for agent turn notification.
// GET /api/v1/agent/wait?timeout=30
func (h *GameProxyHandler) AgentWait(w http.ResponseWriter, r *http.Request) {
	agentID := auth.GetAgentID(r.Context())

	// Parse timeout (default 30s, max 60s)
	timeout := 30 * time.Second
	if t := r.URL.Query().Get("timeout"); t != "" {
		if secs, err := strconv.Atoi(t); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
			if timeout > 60*time.Second {
				timeout = 60 * time.Second
			}
		}
	}

	// Find the agent's active game
	gameID, err := h.gameRepo.FindActiveGameForAgent(r.Context(), agentID)
	if err != nil || gameID == "" {
		w.WriteHeader(http.StatusNoContent) // no active game
		return
	}

	// Check current state — if it's already our turn, return immediately
	var stateResp natsClient.StateResponse
	err = h.nats.RequestJSON(natsClient.SubjectPokerRoomState(gameID), natsClient.StateRequest{
		AgentID: agentID,
	}, &stateResp, natsTimeout)
	if err != nil || !stateResp.Success {
		// Game room may not exist yet or already cleaned up
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Parse state to check valid_actions
	var state map[string]json.RawMessage
	if err := json.Unmarshal(stateResp.State, &state); err == nil {
		if va, ok := state["valid_actions"]; ok {
			var actions []json.RawMessage
			if json.Unmarshal(va, &actions) == nil && len(actions) > 0 {
				// Already our turn — return state immediately
				httputil.JSON(w, http.StatusOK, map[string]interface{}{
					"event":   "your_turn",
					"game_id": gameID,
					"state":   json.RawMessage(stateResp.State),
				})
				return
			}
		}
	}

	// Not our turn yet — subscribe to turn_notify and gameover, wait
	turnCh := make(chan *nats.Msg, 4)
	gameOverCh := make(chan *nats.Msg, 1)

	turnSub, err := h.nats.Conn().ChanSubscribe(natsClient.SubjectPokerTurnNotify(gameID), turnCh)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	defer turnSub.Unsubscribe()

	gameOverSub, err := h.nats.Conn().ChanSubscribe(natsClient.SubjectPokerGameOver(gameID), gameOverCh)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	defer gameOverSub.Unsubscribe()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case msg := <-turnCh:
			var evt natsClient.TurnNotifyEvent
			if err := json.Unmarshal(msg.Data, &evt); err != nil {
				continue
			}
			if evt.AgentID != agentID {
				continue // not our turn
			}
			// It's our turn — fetch fresh state and verify valid_actions
			var freshResp natsClient.StateResponse
			if err := h.nats.RequestJSON(natsClient.SubjectPokerRoomState(gameID), natsClient.StateRequest{
				AgentID: agentID,
			}, &freshResp, natsTimeout); err != nil || !freshResp.Success {
				continue // state fetch failed, keep waiting
			}
			// Verify the state actually has valid_actions (turn hasn't passed)
			var freshState map[string]json.RawMessage
			if err := json.Unmarshal(freshResp.State, &freshState); err != nil {
				continue
			}
			if va, ok := freshState["valid_actions"]; !ok || string(va) == "null" || string(va) == "[]" {
				continue // turn already passed, keep waiting
			}
			httputil.JSON(w, http.StatusOK, map[string]interface{}{
				"event":   "your_turn",
				"game_id": gameID,
				"state":   json.RawMessage(freshResp.State),
			})
			return

		case <-gameOverCh:
			httputil.JSON(w, http.StatusOK, map[string]interface{}{
				"event":   "game_over",
				"game_id": gameID,
			})
			return

		case <-timer.C:
			w.WriteHeader(http.StatusNoContent)
			return

		case <-r.Context().Done():
			return // client disconnected
		}
	}
}
