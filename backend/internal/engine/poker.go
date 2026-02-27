package engine

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/moltgame/backend/internal/game"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/moltgame/backend/internal/room"
	"github.com/nats-io/nats.go"
)

// PokerEngine is the NATS-based poker game engine service.
type PokerEngine struct {
	nc       *natsClient.Client
	manager  *room.Manager
	gameRepo *game.Repository // optional: if set, events are persisted incrementally
	subs     []*nats.Subscription
}

// NewPokerEngine creates a new poker engine.
func NewPokerEngine(nc *natsClient.Client) *PokerEngine {
	mgr := room.NewManager()
	return &PokerEngine{
		nc:      nc,
		manager: mgr,
	}
}

// SetGameRepo enables incremental event persistence.
func (e *PokerEngine) SetGameRepo(repo *game.Repository) {
	e.gameRepo = repo
}

// Start registers all NATS subscriptions and begins serving.
func (e *PokerEngine) Start(ctx context.Context) error {
	// Set callbacks to publish via NATS
	e.manager.OnGameOver = e.onGameOver
	e.manager.OnFlushEvents = e.flushEvents
	e.manager.OnTurnNotify = func(gameID, agentID string) {
		e.nc.PublishJSON(natsClient.SubjectPokerTurnNotify(gameID), natsClient.TurnNotifyEvent{
			GameID: gameID, AgentID: agentID,
		})
	}
	e.manager.OnBroadcast = func(gameID string, rm *room.Room, events interface{}) {
		// Publish events
		if events != nil {
			e.nc.PublishJSON(natsClient.SubjectPokerEvent(gameID), events)
		}
		// Publish spectator state
		if specState, err := rm.GetSpectatorState(); err == nil {
			e.nc.PublishJSON(natsClient.SubjectPokerSpectate(gameID), specState)
		}
		// Publish personalized state to each agent
		for _, pid := range rm.PlayerIDs {
			if state, err := rm.GetState(pid); err == nil {
				e.nc.PublishJSON(natsClient.SubjectPokerState(gameID, pid), state)
			}
		}
	}

	var err error
	var sub *nats.Subscription

	// Request-reply: create room
	sub, err = e.nc.Subscribe(natsClient.SubjectPokerRoomCreate, e.handleCreateRoom)
	if err != nil {
		return err
	}
	e.subs = append(e.subs, sub)

	// Request-reply: list rooms
	sub, err = e.nc.Subscribe(natsClient.SubjectPokerRoomList, e.handleListRooms)
	if err != nil {
		return err
	}
	e.subs = append(e.subs, sub)

	// Wildcard subscriptions for per-room subjects
	sub, err = e.nc.Subscribe("poker.room.*.action", e.handleAction)
	if err != nil {
		return err
	}
	e.subs = append(e.subs, sub)

	sub, err = e.nc.Subscribe("poker.room.*.state", e.handleStateQuery)
	if err != nil {
		return err
	}
	e.subs = append(e.subs, sub)

	sub, err = e.nc.Subscribe("poker.room.*.spectate", e.handleSpectateQuery)
	if err != nil {
		return err
	}
	e.subs = append(e.subs, sub)

	sub, err = e.nc.Subscribe("poker.room.*.cleanup", e.handleCleanup)
	if err != nil {
		return err
	}
	e.subs = append(e.subs, sub)

	slog.Info("poker-engine subscriptions registered")
	return nil
}

// Stop gracefully unsubscribes.
func (e *PokerEngine) Stop() {
	for _, sub := range e.subs {
		sub.Unsubscribe()
	}
	slog.Info("poker-engine stopped")
}

// --- Handlers ---

func (e *PokerEngine) handleCreateRoom(msg *nats.Msg) {
	var req natsClient.CreateRoomRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		respondJSON(msg, natsClient.CreateRoomResponse{Error: "invalid request"})
		return
	}

	if _, err := e.manager.CreatePokerRoom(req.GameID, req.PlayerIDs, req.Seed, req.EntryFee, req.PlayerNames); err != nil {
		slog.Error("create poker room failed", "game_id", req.GameID, "error", err)
		respondJSON(msg, natsClient.CreateRoomResponse{Error: err.Error()})
		return
	}

	slog.Info("poker room created", "game_id", req.GameID, "players", len(req.PlayerIDs))
	respondJSON(msg, natsClient.CreateRoomResponse{Success: true})
}

func (e *PokerEngine) handleAction(msg *nats.Msg) {
	roomID := extractRoomID(msg.Subject) // poker.room.{id}.action
	rm := e.manager.GetRoom(roomID)
	if rm == nil {
		respondJSON(msg, natsClient.ActionResponse{Error: "game not found"})
		return
	}

	var req natsClient.ActionRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		respondJSON(msg, natsClient.ActionResponse{Error: "invalid request"})
		return
	}

	if !rm.HasPlayer(req.AgentID) {
		respondJSON(msg, natsClient.ActionResponse{Error: "not in this game"})
		return
	}

	result, err := rm.SubmitAction(req.AgentID, req.Action)
	if err != nil {
		respondJSON(msg, natsClient.ActionResponse{Error: err.Error()})
		return
	}

	eventsJSON, _ := json.Marshal(result.Events)

	// Respond to the caller
	respondJSON(msg, natsClient.ActionResponse{
		Success:   true,
		Events:    eventsJSON,
		GameOver:  result.GameOver,
		NextActor: result.NextActor,
	})

	// Broadcast events to all subscribers
	e.nc.PublishJSON(natsClient.SubjectPokerEvent(roomID), result.Events)

	// Publish personalized state to each agent
	for _, pid := range rm.PlayerIDs {
		state, err := rm.GetState(pid)
		if err != nil {
			continue
		}
		e.nc.PublishJSON(natsClient.SubjectPokerState(roomID, pid), state)
	}

	// Publish spectator state
	specState, err := rm.GetSpectatorState()
	if err == nil {
		e.nc.PublishJSON(natsClient.SubjectPokerSpectate(roomID), specState)
	}

	// If game over, publish game-over event; otherwise notify next actor
	if result.GameOver {
		e.publishGameOver(roomID, rm)
	} else if result.NextActor != "" {
		e.nc.PublishJSON(natsClient.SubjectPokerTurnNotify(roomID), natsClient.TurnNotifyEvent{
			GameID: roomID, AgentID: result.NextActor,
		})
	}
}

func (e *PokerEngine) handleStateQuery(msg *nats.Msg) {
	roomID := extractRoomID(msg.Subject) // poker.room.{id}.state
	rm := e.manager.GetRoom(roomID)
	if rm == nil {
		respondJSON(msg, natsClient.StateResponse{Error: "game not found"})
		return
	}

	var req natsClient.StateRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		respondJSON(msg, natsClient.StateResponse{Error: "invalid request"})
		return
	}

	state, err := rm.GetState(req.AgentID)
	if err != nil {
		respondJSON(msg, natsClient.StateResponse{Error: err.Error()})
		return
	}

	stateJSON, _ := json.Marshal(state)
	respondJSON(msg, natsClient.StateResponse{Success: true, State: stateJSON})
}

func (e *PokerEngine) handleSpectateQuery(msg *nats.Msg) {
	roomID := extractRoomID(msg.Subject) // poker.room.{id}.spectate
	rm := e.manager.GetRoom(roomID)
	if rm == nil {
		respondJSON(msg, natsClient.StateResponse{Error: "game not found"})
		return
	}

	state, err := rm.GetSpectatorState()
	if err != nil {
		respondJSON(msg, natsClient.StateResponse{Error: err.Error()})
		return
	}

	stateJSON, _ := json.Marshal(state)
	respondJSON(msg, natsClient.StateResponse{Success: true, State: stateJSON})
}

func (e *PokerEngine) handleListRooms(msg *nats.Msg) {
	games := e.manager.ListActiveGames()
	liveGames := make([]natsClient.LiveGameInfo, len(games))
	for i, g := range games {
		liveGames[i] = natsClient.LiveGameInfo{
			GameID:      g.GameID,
			PlayerCount: g.PlayerCount,
			Phase:       g.Phase,
			HandNum:     g.HandNum,
		}
	}
	respondJSON(msg, natsClient.ListRoomsResponse{Games: liveGames})
}

func (e *PokerEngine) handleCleanup(msg *nats.Msg) {
	roomID := extractRoomID(msg.Subject) // poker.room.{id}.cleanup
	e.manager.RemoveRoom(roomID)
	slog.Info("poker room cleaned up", "game_id", roomID)
	respondJSON(msg, natsClient.CreateRoomResponse{Success: true})
}

// flushEvents persists new events to DB (called from timeout handler).
func (e *PokerEngine) flushEvents(gameID string, rm *room.Room) {
	if e.gameRepo == nil {
		return
	}
	startSeq, newEvts := rm.DrainNewEvents()
	if len(newEvts) > 0 {
		if err := e.gameRepo.RecordEvents(context.Background(), gameID, startSeq, newEvts); err != nil {
			slog.Error("failed to persist events", "game_id", gameID, "error", err)
		}
	}
}

// --- Game over ---

func (e *PokerEngine) onGameOver(gameID string, rm *room.Room) {
	// Called by room's turn timer when game ends due to timeout
	e.publishGameOver(gameID, rm)
}

func (e *PokerEngine) publishGameOver(gameID string, rm *room.Room) {
	if rm.Poker == nil || !rm.Poker.Finished {
		return
	}

	rankings, _ := json.Marshal(rm.Poker.GetRankings())
	events := rm.GetAccumulatedEvents()
	eventsJSON, _ := json.Marshal(events)

	evt := natsClient.GameOverEvent{
		GameID:            gameID,
		Rankings:          rankings,
		AccumulatedEvents: eventsJSON,
		EntryFee:          rm.EntryFee,
	}

	if err := e.nc.PublishJSON(natsClient.SubjectPokerGameOver(gameID), evt); err != nil {
		slog.Error("failed to publish game over", "game_id", gameID, "error", err)
	} else {
		slog.Info("poker game over published", "game_id", gameID)
	}
}

// --- Helpers ---

// extractRoomID extracts the room ID from a NATS subject like "poker.room.{id}.action"
func extractRoomID(subject string) string {
	// subject format: poker.room.{roomID}.{suffix}
	// Split by "." and take index 2
	parts := splitSubject(subject)
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

func splitSubject(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func respondJSON(msg *nats.Msg, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal response", "error", err)
		return
	}
	if err := msg.Respond(payload); err != nil {
		slog.Error("failed to respond", "error", err)
	}
}

// ListActiveGames is exposed for the manager to also use directly from ActiveGameInfo.
func (e *PokerEngine) ListActiveGames() []room.ActiveGameInfo {
	return e.manager.ListActiveGames()
}
