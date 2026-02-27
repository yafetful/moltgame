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
	turnTimeout        = 30 * time.Second
	disconnectThreshold = 3 // consecutive timeouts before marking disconnected
	handBreakDuration   = 5 * time.Second
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

	// Hand break timer: delays between hands for animation
	handBreakTimer *time.Timer
	NextHandAt     *time.Time
	OnBroadcast    func(gameID string, room *Room, events interface{}) // called to broadcast state updates (called outside lock)
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
	OnBroadcast   func(gameID string, room *Room, events interface{})
}

// NewManager creates a new room manager.
func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

// CreatePokerRoom creates a new poker game room.
func (m *Manager) CreatePokerRoom(gameID string, playerIDs []string, seed int64, entryFee int, playerNames map[string]string) (*Room, error) {
	g := poker.NewGame(gameID, playerIDs, seed, playerNames)

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
	room.OnBroadcast = m.OnBroadcast

	// Start the first hand and accumulate its events
	firstHandEvents, _ := g.StartHand()
	for _, evt := range firstHandEvents {
		payload, _ := json.Marshal(evt.Data)
		room.Events = append(room.Events, models.GameEvent{
			GameID:    gameID,
			SeqNum:    len(room.Events) + 1,
			EventType: string(evt.Type),
			Payload:   payload,
			CreatedAt: time.Now(),
		})
	}

	// Persist initial events before publishing room (prevents race with timer)
	if room.OnFlushEvents != nil {
		room.OnFlushEvents(gameID, room)
	}

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

	// Persist events while holding lock (prevents race with timer callbacks)
	if r.OnFlushEvents != nil {
		r.OnFlushEvents(r.GameID, r)
	}

	// Manage turn timer after successful action
	if result.GameOver {
		r.StopTurnTimer()
	} else if r.Poker.Phase == poker.PhaseIdle {
		// Hand ended but game continues — schedule break before next hand
		r.StopTurnTimer()
		r.scheduleHandBreak()
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
	state := r.Poker.GetGameState(playerID)
	state.NextHandAt = r.NextHandAt
	return state, nil
}

// GetSpectatorState returns the god-view state.
func (r *Room) GetSpectatorState() (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Poker == nil {
		return nil, fmt.Errorf("game not initialized")
	}
	state := r.Poker.GetSpectatorState()
	state.NextHandAt = r.NextHandAt
	return state, nil
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

// scheduleHandBreak schedules the next hand to start after a delay.
// Must be called with r.mu held.
func (r *Room) scheduleHandBreak() {
	if r.handBreakTimer != nil {
		r.handBreakTimer.Stop()
	}
	t := time.Now().Add(handBreakDuration)
	r.NextHandAt = &t
	r.handBreakTimer = time.AfterFunc(handBreakDuration, r.startNextHand)
}

// startNextHand is fired by the hand break timer. It starts the next hand
// and broadcasts the resulting events.
func (r *Room) startNextHand() {
	r.mu.Lock()

	r.NextHandAt = nil
	r.handBreakTimer = nil

	if r.Poker == nil || r.Poker.Finished {
		r.mu.Unlock()
		return
	}

	events, err := r.Poker.StartHand()
	if err != nil {
		slog.Error("startNextHand failed", "game_id", r.GameID, "error", err)
		r.mu.Unlock()
		return
	}

	// Accumulate events
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

	// Persist
	if r.OnFlushEvents != nil {
		r.OnFlushEvents(r.GameID, r)
	}

	// Determine next state
	var gameOver bool
	var nextActor string
	if r.Poker.Finished {
		r.Status = models.GameStatusFinished
		gameOver = true
	} else if r.Poker.Phase == poker.PhaseIdle {
		// Instant resolution (e.g. blinds all-in) — schedule another break
		r.scheduleHandBreak()
	} else {
		r.ResetTurnTimer()
		nextActor = r.Poker.CurrentActor()
	}

	r.mu.Unlock()

	// Broadcast outside lock
	if r.OnBroadcast != nil {
		r.OnBroadcast(r.GameID, r, events)
	}

	if gameOver {
		if r.OnGameOver != nil {
			r.OnGameOver(r.GameID, r)
		}
	} else if nextActor != "" {
		if r.OnTurnNotify != nil {
			r.OnTurnNotify(r.GameID, nextActor)
		}
	}
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

// StopTurnTimer stops the turn timer and any hand break timer.
func (r *Room) StopTurnTimer() {
	if r.turnTimer != nil {
		r.turnTimer.Stop()
		r.turnTimer = nil
	}
	if r.handBreakTimer != nil {
		r.handBreakTimer.Stop()
		r.handBreakTimer = nil
	}
	r.NextHandAt = nil
}

// handleTimeout submits a default action for the current actor.
func (r *Room) handleTimeout() {
	r.mu.Lock()

	if r.Poker == nil || r.Poker.Finished {
		r.mu.Unlock()
		return
	}

	actor := r.Poker.CurrentActor()
	if actor == "" {
		r.mu.Unlock()
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

	result, err := r.submitPokerAction(actor, mustJSON(action))
	if err != nil {
		slog.Error("timeout action failed", "game_id", r.GameID, "error", err)
		r.mu.Unlock()
		return
	}

	// Persist new events from timeout action
	if r.OnFlushEvents != nil {
		r.OnFlushEvents(r.GameID, r)
	}

	// Determine next state
	var gameOver bool
	var nextActor string
	if r.Poker.Finished {
		gameOver = true
	} else if r.Poker.Phase == poker.PhaseIdle {
		// Hand ended — schedule break before next hand
		r.scheduleHandBreak()
	} else {
		nextActor = r.Poker.CurrentActor()
		r.ResetTurnTimer()
	}

	r.mu.Unlock()

	// Broadcast outside lock
	if r.OnBroadcast != nil && result != nil {
		r.OnBroadcast(r.GameID, r, result.Events)
	}

	if gameOver {
		if r.OnGameOver != nil {
			r.OnGameOver(r.GameID, r)
		}
	} else if nextActor != "" {
		if r.OnTurnNotify != nil {
			r.OnTurnNotify(r.GameID, nextActor)
		}
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
