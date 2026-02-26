package room

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moltgame/backend/internal/models"
	"github.com/moltgame/backend/internal/poker"
)

const (
	turnTimeout          = 30 * time.Second
	disconnectThreshold  = 3 // consecutive timeouts before marking disconnected
)

// Room wraps an active game instance.
type Room struct {
	mu sync.RWMutex

	GameID    string
	GameType  models.GameType
	Status    models.GameStatus
	PlayerIDs []string // agent IDs in seat order
	CreatedAt time.Time
	StartedAt *time.Time

	// Accumulated events for persistence (Event Sourcing)
	Events       []models.GameEvent
	flushedCount int // number of events already persisted to DB

	// Poker game instance (only non-nil for poker rooms)
	Poker    *poker.Game
	EntryFee int // entry fee for settlement

	// Turn timer: auto-submits default action on timeout
	turnTimer     *time.Timer
	OnGameOver    func(gameID string, room *Room)         // called when game ends (including by timeout)
	OnFlushEvents func(gameID string, room *Room)         // called after events are accumulated to persist them
	OnTurnNotify  func(gameID string, agentID string)     // called to notify next actor it's their turn
}

// Manager manages all active game rooms in memory.
type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room // gameID → room

	// OnGameOver is called when a game ends due to timeout.
	// For normal actions, the API layer handles settlement directly.
	OnGameOver    func(gameID string, room *Room)
	OnFlushEvents func(gameID string, room *Room)
	OnTurnNotify  func(gameID string, agentID string)
}

// NewManager creates a new room manager.
func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

// CreatePokerRoom creates a new poker game room.
func (m *Manager) CreatePokerRoom(gameID string, playerIDs []string, seed int64, entryFee int) (*Room, error) {
	g := poker.NewGame(gameID, playerIDs, seed)

	room := &Room{
		GameID:    gameID,
		GameType:  models.GameTypePoker,
		Status:    models.GameStatusPlaying,
		PlayerIDs: playerIDs,
		EntryFee:  entryFee,
		CreatedAt: time.Now(),
	}
	now := time.Now()
	room.StartedAt = &now
	room.Poker = g
	room.OnGameOver = m.OnGameOver
	room.OnFlushEvents = m.OnFlushEvents
	room.OnTurnNotify = m.OnTurnNotify

	// Start the first hand
	g.StartHand()

	m.mu.Lock()
	m.rooms[gameID] = room
	m.mu.Unlock()

	// Start turn timer for the first actor
	room.ResetTurnTimer()

	return room, nil
}

// GetRoom returns a room by game ID.
func (m *Manager) GetRoom(gameID string) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rooms[gameID]
}

// RemoveRoom removes a finished room from memory.
func (m *Manager) RemoveRoom(gameID string) {
	m.mu.Lock()
	if r, ok := m.rooms[gameID]; ok {
		r.StopTurnTimer()
		delete(m.rooms, gameID)
	}
	m.mu.Unlock()
}

// ListActiveGames returns a summary of all active games.
func (m *Manager) ListActiveGames() []ActiveGameInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ActiveGameInfo, 0, len(m.rooms))
	for _, room := range m.rooms {
		info := ActiveGameInfo{
			GameID:      room.GameID,
			GameType:    room.GameType,
			PlayerCount: len(room.PlayerIDs),
			CreatedAt:   room.CreatedAt,
		}
		room.mu.RLock()
		if room.Poker != nil {
			info.Phase = room.Poker.Phase.String()
			info.HandNum = room.Poker.HandNum
		}
		room.mu.RUnlock()
		result = append(result, info)
	}
	return result
}

// ActiveGameInfo is summary info about an active game.
type ActiveGameInfo struct {
	GameID      string          `json:"game_id"`
	GameType    models.GameType `json:"game_type"`
	PlayerCount int             `json:"player_count"`
	Phase       string          `json:"phase"`
	HandNum     int             `json:"hand_num,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// SubmitAction processes a player action in the specified game room.
func (r *Room) SubmitAction(playerID string, actionJSON json.RawMessage) (*ActionResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.GameType != models.GameTypePoker {
		return nil, fmt.Errorf("unsupported game type: %s", r.GameType)
	}

	result, err := r.submitPokerAction(playerID, actionJSON)
	if err != nil {
		return nil, err
	}

	// Manage turn timer after successful action
	if result.GameOver {
		r.StopTurnTimer()
	} else {
		r.ResetTurnTimer()
	}

	return result, nil
}

// GetState returns the game state for a specific player.
func (r *Room) GetState(playerID string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Poker == nil {
		return nil, fmt.Errorf("game not initialized")
	}
	return r.Poker.GetGameState(playerID), nil
}

// GetSpectatorState returns the god-view state.
func (r *Room) GetSpectatorState() (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Poker == nil {
		return nil, fmt.Errorf("game not initialized")
	}
	return r.Poker.GetSpectatorState(), nil
}

// IsFinished returns whether the game has ended.
func (r *Room) IsFinished() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Poker != nil && r.Poker.Finished
}

// ActionResult contains the result of processing an action.
type ActionResult struct {
	Events    interface{} `json:"events,omitempty"`
	GameOver  bool        `json:"game_over"`
	NextActor string      `json:"next_actor,omitempty"`
}

func (r *Room) submitPokerAction(playerID string, actionJSON json.RawMessage) (*ActionResult, error) {
	var action poker.Action
	if err := json.Unmarshal(actionJSON, &action); err != nil {
		return nil, fmt.Errorf("invalid action format: %w", err)
	}

	events, err := r.Poker.Act(playerID, action)
	if err != nil {
		return nil, err
	}

	// Accumulate events for persistence
	for _, evt := range events {
		payload, _ := json.Marshal(evt.Data)
		r.Events = append(r.Events, models.GameEvent{
			GameID:    r.GameID,
			SeqNum:    len(r.Events) + 1,
			EventType: string(evt.Type),
			Payload:   payload,
			CreatedAt: time.Now(),
		})
	}

	// Auto-start next hand if current hand ended but game isn't over.
	// Loop because a new hand may resolve immediately (e.g., both players
	// all-in from blinds), leaving the phase idle again.
	for r.Poker.Phase == poker.PhaseIdle && !r.Poker.Finished {
		nextEvents, startErr := r.Poker.StartHand()
		if startErr != nil {
			break
		}
		events = append(events, nextEvents...)
		for _, evt := range nextEvents {
			payload, _ := json.Marshal(evt.Data)
			r.Events = append(r.Events, models.GameEvent{
				GameID:    r.GameID,
				SeqNum:    len(r.Events) + 1,
				EventType: string(evt.Type),
				Payload:   payload,
				CreatedAt: time.Now(),
			})
		}
	}

	result := &ActionResult{
		Events:   events,
		GameOver: r.Poker.Finished,
	}

	if !r.Poker.Finished {
		result.NextActor = r.Poker.CurrentActor()
	}

	if r.Poker.Finished {
		r.Status = models.GameStatusFinished
	}

	return result, nil
}

// GetAccumulatedEvents returns a copy of all accumulated events.
func (r *Room) GetAccumulatedEvents() []models.GameEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	evts := make([]models.GameEvent, len(r.Events))
	copy(evts, r.Events)
	return evts
}

// DrainNewEvents returns events that haven't been flushed yet and advances the cursor.
// Caller must hold r.mu (write lock) or call this while already locked.
func (r *Room) DrainNewEvents() (startSeq int, events []models.GameEvent) {
	if r.flushedCount >= len(r.Events) {
		return r.flushedCount + 1, nil
	}
	startSeq = r.flushedCount + 1
	events = make([]models.GameEvent, len(r.Events)-r.flushedCount)
	copy(events, r.Events[r.flushedCount:])
	r.flushedCount = len(r.Events)
	return startSeq, events
}

// ResetTurnTimer resets the turn timer. Should be called after each action.
// Disconnected players get an immediate timeout (no 30s wait).
func (r *Room) ResetTurnTimer() {
	if r.turnTimer != nil {
		r.turnTimer.Stop()
	}

	if r.Poker == nil || r.Poker.Finished {
		return
	}

	actor := r.Poker.CurrentActor()
	if actor == "" {
		return
	}

	// Disconnected players: auto-fold immediately (short delay to avoid stack overflow)
	p := r.Poker.PlayerByID(actor)
	if p != nil && p.Disconnected {
		r.turnTimer = time.AfterFunc(100*time.Millisecond, func() {
			r.handleTimeout()
		})
		return
	}

	r.turnTimer = time.AfterFunc(turnTimeout, func() {
		r.handleTimeout()
	})
}

// StopTurnTimer stops the turn timer.
func (r *Room) StopTurnTimer() {
	if r.turnTimer != nil {
		r.turnTimer.Stop()
		r.turnTimer = nil
	}
}

// handleTimeout submits a default action for the current actor.
func (r *Room) handleTimeout() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Poker == nil || r.Poker.Finished {
		return
	}

	actor := r.Poker.CurrentActor()
	if actor == "" {
		return
	}

	// Track consecutive timeouts for the player
	p := r.Poker.PlayerByID(actor)
	if p != nil {
		p.TimeoutCount++
		if p.TimeoutCount >= disconnectThreshold && !p.Disconnected {
			p.Disconnected = true
			slog.Warn("player disconnected", "game_id", r.GameID, "player", actor, "timeouts", p.TimeoutCount)
			// Accumulate disconnect event
			payload, _ := json.Marshal(poker.PlayerDisconnectedData{
				Seat:     p.Seat,
				PlayerID: p.ID,
			})
			r.Events = append(r.Events, models.GameEvent{
				GameID:    r.GameID,
				SeqNum:    len(r.Events) + 1,
				EventType: string(poker.EventPlayerDisconnected),
				Payload:   payload,
				CreatedAt: time.Now(),
			})
		}
	}

	// Disconnected players always fold; others check if possible
	action := poker.Action{Type: poker.ActionFold}
	if p == nil || !p.Disconnected {
		state := r.Poker.GetGameState(actor)
		for _, opt := range state.ValidActions {
			if opt.Type == poker.ActionCheck {
				action = poker.Action{Type: poker.ActionCheck}
				break
			}
		}
	}

	slog.Info("turn timeout", "game_id", r.GameID, "player", actor, "default_action", action.Type, "disconnected", p != nil && p.Disconnected)

	_, err := r.submitPokerAction(actor, mustJSON(action))
	if err != nil {
		slog.Error("timeout action failed", "game_id", r.GameID, "error", err)
	}

	// Persist new events from timeout action
	if r.OnFlushEvents != nil {
		r.OnFlushEvents(r.GameID, r)
	}

	// Check if game ended due to timeout action
	if r.Poker.Finished {
		if r.OnGameOver != nil {
			go r.OnGameOver(r.GameID, r)
		}
	} else {
		// Notify next actor it's their turn
		if r.OnTurnNotify != nil {
			if nextActor := r.Poker.CurrentActor(); nextActor != "" {
				go r.OnTurnNotify(r.GameID, nextActor)
			}
		}
		r.ResetTurnTimer()
	}
}

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// HasPlayer returns whether the given player is in this room.
func (r *Room) HasPlayer(playerID string) bool {
	for _, pid := range r.PlayerIDs {
		if pid == playerID {
			return true
		}
	}
	return false
}
