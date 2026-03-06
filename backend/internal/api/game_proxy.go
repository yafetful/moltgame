package api

import (
	"context"
	"encoding/json"
	"fmt"
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
	agentRepo  *auth.AgentRepository
	settlement *gameRepo.SettlementService
}

// NewGameProxyHandler creates a new game proxy handler.
func NewGameProxyHandler(nc *natsClient.Client, repo *gameRepo.Repository, agentRepo *auth.AgentRepository, settlement *gameRepo.SettlementService) *GameProxyHandler {
	return &GameProxyHandler{
		nats:       nc,
		gameRepo:   repo,
		agentRepo:  agentRepo,
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

	// Look up agent names for display
	playerNames := make(map[string]string)
	for _, id := range req.PlayerIDs {
		if agent, err := h.agentRepo.GetAgentByID(r.Context(), id); err == nil {
			playerNames[id] = agent.Name
		}
	}

	// Create room via NATS
	seed := cryptoSeed()
	var resp natsClient.CreateRoomResponse
	err = h.nats.RequestJSON(natsClient.SubjectPokerRoomCreate, natsClient.CreateRoomRequest{
		GameID:      dbGame.ID,
		PlayerIDs:   req.PlayerIDs,
		PlayerNames: playerNames,
		Seed:        seed,
		EntryFee:    req.EntryFee,
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
		// Fetch current valid actions to help the agent recover
		errResp := map[string]interface{}{
			"error": resp.Error,
			"code":  "invalid_action",
		}
		var stateResp natsClient.StateResponse
		if err := h.nats.RequestJSON(natsClient.SubjectPokerRoomState(gameID), natsClient.StateRequest{
			AgentID: agentID,
		}, &stateResp, natsTimeout); err == nil && stateResp.Success {
			var state map[string]json.RawMessage
			if json.Unmarshal(stateResp.State, &state) == nil {
				if va, ok := state["valid_actions"]; ok {
					errResp["valid_actions"] = json.RawMessage(va)
				}
			}
		}
		httputil.JSON(w, http.StatusBadRequest, errResp)
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
// For live games, proxies via NATS. For finished games (room cleaned up), rebuilds from DB events.
// GET /api/v1/games/{id}/spectate
func (h *GameProxyHandler) GetSpectatorState(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")

	// Try live game first via NATS
	var resp natsClient.StateResponse
	err := h.nats.RequestJSON(natsClient.SubjectPokerRoomSpectate(gameID), struct{}{}, &resp, natsTimeout)
	if err == nil && resp.Success {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp.State)
		return
	}

	// NATS failed — check if this is a finished game in DB
	stateJSON, err := h.rebuildFinishedGameState(r.Context(), gameID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "game_not_found", "Game not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(stateJSON)
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
// Handles two scenarios:
// 1. Agent has an active game → wait for turn or game_over
// 2. Agent has no active game → wait for match_found from matchmaking
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
		// No active game — wait for match_found
		h.waitForMatch(w, r, agentID, timeout)
		return
	}

	// Has active game — wait for turn
	h.waitForTurn(w, r, agentID, gameID, timeout)
}

// waitForMatch waits for a matchmaking match_found event for this agent.
func (h *GameProxyHandler) waitForMatch(w http.ResponseWriter, r *http.Request, agentID string, timeout time.Duration) {
	// Subscribe to all matchmaking notifications
	matchCh := make(chan *nats.Msg, 4)
	matchSub, err := h.nats.Conn().ChanSubscribe("system.matchmaking.>", matchCh)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	defer matchSub.Unsubscribe()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case msg := <-matchCh:
			var matchMsg natsClient.MatchFoundMsg
			if err := json.Unmarshal(msg.Data, &matchMsg); err != nil {
				continue
			}
			// Check if this agent is in the match
			for _, id := range matchMsg.PlayerIDs {
				if id == agentID {
					resp := map[string]interface{}{
						"event":         "match_found",
						"game_id":       matchMsg.GameID,
						"game_type":     matchMsg.GameType,
						"players_count": len(matchMsg.PlayerIDs),
					}
					// Look up player names
					var playerNames []string
					for _, pid := range matchMsg.PlayerIDs {
						if agent, err := h.agentRepo.GetAgentByID(r.Context(), pid); err == nil {
							playerNames = append(playerNames, agent.Name)
						}
					}
					if len(playerNames) > 0 {
						resp["players"] = playerNames
					}
					httputil.JSON(w, http.StatusOK, resp)
					return
				}
			}

		case <-timer.C:
			w.WriteHeader(http.StatusNoContent)
			return

		case <-r.Context().Done():
			return
		}
	}
}

// waitForTurn waits for the agent's turn or game over in an active game.
func (h *GameProxyHandler) waitForTurn(w http.ResponseWriter, r *http.Request, agentID, gameID string, timeout time.Duration) {
	// Check current state — if it's already our turn, return immediately
	var stateResp natsClient.StateResponse
	err := h.nats.RequestJSON(natsClient.SubjectPokerRoomState(gameID), natsClient.StateRequest{
		AgentID: agentID,
	}, &stateResp, natsTimeout)
	if err != nil || !stateResp.Success {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Parse state to check valid_actions
	var state map[string]json.RawMessage
	if err := json.Unmarshal(stateResp.State, &state); err == nil {
		if va, ok := state["valid_actions"]; ok {
			var actions []json.RawMessage
			if json.Unmarshal(va, &actions) == nil && len(actions) > 0 {
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
				continue
			}
			var freshResp natsClient.StateResponse
			if err := h.nats.RequestJSON(natsClient.SubjectPokerRoomState(gameID), natsClient.StateRequest{
				AgentID: agentID,
			}, &freshResp, natsTimeout); err != nil || !freshResp.Success {
				continue
			}
			var freshState map[string]json.RawMessage
			if err := json.Unmarshal(freshResp.State, &freshState); err != nil {
				continue
			}
			if va, ok := freshState["valid_actions"]; !ok || string(va) == "null" || string(va) == "[]" {
				continue
			}
			httputil.JSON(w, http.StatusOK, map[string]interface{}{
				"event":   "your_turn",
				"game_id": gameID,
				"state":   json.RawMessage(freshResp.State),
			})
			return

		case msg := <-gameOverCh:
			resp := map[string]interface{}{
				"event":   "game_over",
				"game_id": gameID,
			}
			// Enrich with ranking info from the NATS event
			var evt natsClient.GameOverEvent
			if err := json.Unmarshal(msg.Data, &evt); err == nil {
				var rankings []struct {
					Rank     int    `json:"rank"`
					PlayerID string `json:"player_id"`
				}
				if json.Unmarshal(evt.Rankings, &rankings) == nil {
					resp["players_count"] = len(rankings)
					for _, r := range rankings {
						if r.PlayerID == agentID {
							resp["your_rank"] = r.Rank
							break
						}
					}
				}
			}
			httputil.JSON(w, http.StatusOK, resp)
			return

		case <-timer.C:
			w.WriteHeader(http.StatusNoContent)
			return

		case <-r.Context().Done():
			return
		}
	}
}

// GetAgentHistory returns the authenticated agent's game history.
// GET /api/v1/agents/me/history
func (h *GameProxyHandler) GetAgentHistory(w http.ResponseWriter, r *http.Request) {
	agentID := auth.GetAgentID(r.Context())

	history, err := h.gameRepo.GetAgentHistory(r.Context(), agentID, 50)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "query_failed", "Failed to fetch history")
		return
	}
	if history == nil {
		history = []gameRepo.AgentGameHistory{}
	}
	httputil.JSON(w, http.StatusOK, history)
}

// rebuildFinishedGameState reconstructs the final game state from DB events.
// Used when a finished game's room has been cleaned up from poker-engine.
func (h *GameProxyHandler) rebuildFinishedGameState(ctx context.Context, gameID string) (json.RawMessage, error) {
	// Verify game exists and is finished
	game, err := h.gameRepo.GetGame(ctx, gameID)
	if err != nil || game.Status != models.GameStatusFinished {
		return nil, fmt.Errorf("game not found or not finished")
	}

	events, err := h.gameRepo.GetGameEvents(ctx, gameID)
	if err != nil || len(events) == 0 {
		return nil, fmt.Errorf("no events found")
	}

	// Walk events to extract final state
	type playerInfo struct {
		ID         string   `json:"id"`
		Name       string   `json:"name,omitempty"`
		Seat       int      `json:"seat"`
		Chips      int      `json:"chips"`
		Hole       []string `json:"hole,omitempty"`
		Folded     bool     `json:"folded"`
		AllIn      bool     `json:"all_in"`
		Eliminated bool     `json:"eliminated"`
	}

	players := make(map[int]*playerInfo)    // seat → info
	var community []string
	var handNum, dealerSeat, smallBlind, bigBlind int
	var lastPhase string

	for _, evt := range events {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			continue
		}

		switch evt.EventType {
		case "hand_start":
			// Reset per-hand state
			for _, p := range players {
				p.Folded = false
				p.AllIn = false
				p.Hole = nil
			}
			community = nil
			lastPhase = "preflop"

			if v, ok := payload["hand_num"]; ok {
				json.Unmarshal(v, &handNum)
			}
			if v, ok := payload["dealer_seat"]; ok {
				json.Unmarshal(v, &dealerSeat)
			}
			if v, ok := payload["small_blind"]; ok {
				json.Unmarshal(v, &smallBlind)
			}
			if v, ok := payload["big_blind"]; ok {
				json.Unmarshal(v, &bigBlind)
			}
			if v, ok := payload["players"]; ok {
				var pList []struct {
					ID    string `json:"id"`
					Seat  int    `json:"seat"`
					Chips int    `json:"chips"`
				}
				if json.Unmarshal(v, &pList) == nil {
					for _, p := range pList {
						if players[p.Seat] == nil {
							players[p.Seat] = &playerInfo{}
						}
						players[p.Seat].ID = p.ID
						players[p.Seat].Seat = p.Seat
						players[p.Seat].Chips = p.Chips
					}
				}
			}

		case "hole_dealt":
			var seat int
			var cards []string
			if v, ok := payload["seat"]; ok {
				json.Unmarshal(v, &seat)
			}
			if v, ok := payload["cards"]; ok {
				json.Unmarshal(v, &cards)
			}
			if p := players[seat]; p != nil {
				p.Hole = cards
			}

		case "community_dealt":
			if v, ok := payload["board"]; ok {
				json.Unmarshal(v, &community)
			}
			if v, ok := payload["phase"]; ok {
				json.Unmarshal(v, &lastPhase)
			}

		case "player_action":
			var seat int
			var action string
			if v, ok := payload["seat"]; ok {
				json.Unmarshal(v, &seat)
			}
			if v, ok := payload["action"]; ok {
				json.Unmarshal(v, &action)
			}
			if v, ok := payload["chips_left"]; ok {
				if p := players[seat]; p != nil {
					json.Unmarshal(v, &p.Chips)
				}
			}
			if p := players[seat]; p != nil {
				switch action {
				case "fold":
					p.Folded = true
				case "allin":
					p.AllIn = true
				}
			}

		case "showdown":
			lastPhase = "showdown"
			if v, ok := payload["board"]; ok {
				json.Unmarshal(v, &community)
			}
			if v, ok := payload["players"]; ok {
				var sdPlayers []struct {
					Seat int      `json:"seat"`
					Hole []string `json:"hole"`
				}
				if json.Unmarshal(v, &sdPlayers) == nil {
					for _, sp := range sdPlayers {
						if p := players[sp.Seat]; p != nil {
							p.Hole = sp.Hole
						}
					}
				}
			}

		case "pot_awarded":
			if v, ok := payload["winners"]; ok {
				var winners []struct {
					Seat   int `json:"seat"`
					Amount int `json:"amount"`
				}
				if json.Unmarshal(v, &winners) == nil {
					for _, w := range winners {
						if p := players[w.Seat]; p != nil {
							p.Chips += w.Amount
						}
					}
				}
			}

		case "player_eliminated":
			var seat int
			if v, ok := payload["seat"]; ok {
				json.Unmarshal(v, &seat)
			}
			if p := players[seat]; p != nil {
				p.Eliminated = true
			}

		case "hand_end":
			if v, ok := payload["players"]; ok {
				var pList []struct {
					ID    string `json:"id"`
					Seat  int    `json:"seat"`
					Chips int    `json:"chips"`
				}
				if json.Unmarshal(v, &pList) == nil {
					for _, p := range pList {
						if players[p.Seat] != nil {
							players[p.Seat].Chips = p.Chips
						}
					}
				}
			}
		}
	}

	// Look up agent names
	for _, p := range players {
		if p.ID != "" && h.agentRepo != nil {
			if agent, err := h.agentRepo.GetAgentByID(ctx, p.ID); err == nil {
				p.Name = agent.Name
			}
		}
	}

	// Build sorted player list
	playerList := make([]*playerInfo, 0, len(players))
	for seat := 0; seat < 6; seat++ {
		if p, ok := players[seat]; ok {
			playerList = append(playerList, p)
		}
	}

	state := map[string]interface{}{
		"game_id":     gameID,
		"hand_num":    handNum,
		"phase":       lastPhase,
		"finished":    true,
		"community":   community,
		"current_bet": 0,
		"small_blind": smallBlind,
		"big_blind":   bigBlind,
		"dealer_seat": dealerSeat,
		"pots":        []interface{}{},
		"action_on":   -1,
		"players":     playerList,
	}

	return json.Marshal(state)
}


