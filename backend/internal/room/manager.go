package room

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moltgame/backend/internal/models"
	"github.com/moltgame/backend/internal/poker"
	"github.com/moltgame/backend/internal/werewolf"
)

const turnTimeout = 30 * time.Second

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
	Events []models.GameEvent

	// Exactly one of these is non-nil
	Poker    *poker.Game
	Werewolf *werewolf.Game

	// Turn timer: auto-submits default action on timeout
	turnTimer  *time.Timer
	OnGameOver func(gameID string, room *Room) // called when game ends (including by timeout)
}

// Manager manages all active game rooms in memory.
type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room // gameID → room

	// OnGameOver is called when a game ends due to timeout.
	// For normal actions, the API layer handles settlement directly.
	OnGameOver func(gameID string, room *Room)
}

// NewManager creates a new room manager.
func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

// CreatePokerRoom creates a new poker game room.
func (m *Manager) CreatePokerRoom(gameID string, playerIDs []string, seed int64) (*Room, error) {
	g := poker.NewGame(gameID, playerIDs, seed)

	room := &Room{
		GameID:    gameID,
		GameType:  models.GameTypePoker,
		Status:    models.GameStatusPlaying,
		PlayerIDs: playerIDs,
		CreatedAt: time.Now(),
	}
	now := time.Now()
	room.StartedAt = &now
	room.Poker = g
	room.OnGameOver = m.OnGameOver

	// Start the first hand
	g.StartHand()

	m.mu.Lock()
	m.rooms[gameID] = room
	m.mu.Unlock()

	// Start turn timer for the first actor
	room.ResetTurnTimer()

	return room, nil
}

// CreateWerewolfRoom creates a new werewolf game room.
func (m *Manager) CreateWerewolfRoom(gameID string, playerIDs []string, seed int64) (*Room, []werewolf.Event, error) {
	g, err := werewolf.NewGame(gameID, playerIDs, seed)
	if err != nil {
		return nil, nil, fmt.Errorf("create werewolf game: %w", err)
	}

	events, err := g.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("start werewolf game: %w", err)
	}

	room := &Room{
		GameID:    gameID,
		GameType:  models.GameTypeWerewolf,
		Status:    models.GameStatusPlaying,
		PlayerIDs: playerIDs,
		CreatedAt: time.Now(),
	}
	now := time.Now()
	room.StartedAt = &now
	room.Werewolf = g
	room.OnGameOver = m.OnGameOver

	m.mu.Lock()
	m.rooms[gameID] = room
	m.mu.Unlock()

	// Start turn timer for the first actor
	room.ResetTurnTimer()

	return room, events, nil
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
		if room.Werewolf != nil {
			info.Phase = room.Werewolf.Phase.String()
			info.Day = room.Werewolf.Day
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
	Day         int             `json:"day,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// SubmitAction processes a player action in the specified game room.
func (r *Room) SubmitAction(playerID string, actionJSON json.RawMessage) (*ActionResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result *ActionResult
	var err error

	switch r.GameType {
	case models.GameTypePoker:
		result, err = r.submitPokerAction(playerID, actionJSON)
	case models.GameTypeWerewolf:
		result, err = r.submitWerewolfAction(playerID, actionJSON)
	default:
		return nil, fmt.Errorf("unknown game type: %s", r.GameType)
	}

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

	switch r.GameType {
	case models.GameTypePoker:
		if r.Poker == nil {
			return nil, fmt.Errorf("game not initialized")
		}
		return r.Poker.GetGameState(playerID), nil
	case models.GameTypeWerewolf:
		if r.Werewolf == nil {
			return nil, fmt.Errorf("game not initialized")
		}
		return r.Werewolf.GetGameState(playerID), nil
	default:
		return nil, fmt.Errorf("unknown game type: %s", r.GameType)
	}
}

// GetSpectatorState returns the god-view state.
func (r *Room) GetSpectatorState() (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	switch r.GameType {
	case models.GameTypePoker:
		if r.Poker == nil {
			return nil, fmt.Errorf("game not initialized")
		}
		return r.Poker.GetSpectatorState(), nil
	case models.GameTypeWerewolf:
		if r.Werewolf == nil {
			return nil, fmt.Errorf("game not initialized")
		}
		return r.Werewolf.GetSpectatorState(), nil
	default:
		return nil, fmt.Errorf("unknown game type: %s", r.GameType)
	}
}

// IsFinished returns whether the game has ended.
func (r *Room) IsFinished() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	switch r.GameType {
	case models.GameTypePoker:
		return r.Poker != nil && r.Poker.Finished
	case models.GameTypeWerewolf:
		return r.Werewolf != nil && r.Werewolf.IsGameOver()
	}
	return false
}

// ActionResult contains the result of processing an action.
type ActionResult struct {
	Events     interface{} `json:"events,omitempty"`
	GameOver   bool        `json:"game_over"`
	NextActor  string      `json:"next_actor,omitempty"`
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
		})
	}

	// Auto-start next hand if current hand ended but game isn't over
	if r.Poker.Phase == poker.PhaseIdle && !r.Poker.Finished {
		nextEvents, startErr := r.Poker.StartHand()
		if startErr == nil {
			events = append(events, nextEvents...)
			for _, evt := range nextEvents {
				payload, _ := json.Marshal(evt.Data)
				r.Events = append(r.Events, models.GameEvent{
					GameID:    r.GameID,
					SeqNum:    len(r.Events) + 1,
					EventType: string(evt.Type),
					Payload:   payload,
				})
			}
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

func (r *Room) submitWerewolfAction(playerID string, actionJSON json.RawMessage) (*ActionResult, error) {
	var action werewolf.Action
	if err := json.Unmarshal(actionJSON, &action); err != nil {
		return nil, fmt.Errorf("invalid action format: %w", err)
	}

	events, err := r.Werewolf.Act(playerID, action)
	if err != nil {
		return nil, err
	}

	// Accumulate events for persistence
	for _, evt := range events {
		payload, _ := json.Marshal(evt)
		r.Events = append(r.Events, models.GameEvent{
			GameID:    r.GameID,
			SeqNum:    len(r.Events) + 1,
			EventType: string(evt.Type),
			Payload:   payload,
		})
	}

	result := &ActionResult{
		Events:   events,
		GameOver: r.Werewolf.IsGameOver(),
	}

	if !r.Werewolf.IsGameOver() {
		result.NextActor = r.Werewolf.CurrentActor()
	}

	if r.Werewolf.IsGameOver() {
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

// ResetTurnTimer resets the turn timer. Should be called after each action.
func (r *Room) ResetTurnTimer() {
	if r.turnTimer != nil {
		r.turnTimer.Stop()
	}

	// Check if any actor needs to act
	var actor string
	switch r.GameType {
	case models.GameTypePoker:
		if r.Poker != nil && !r.Poker.Finished {
			actor = r.Poker.CurrentActor()
		}
	case models.GameTypeWerewolf:
		if r.Werewolf != nil && !r.Werewolf.IsGameOver() {
			actor = r.Werewolf.CurrentActor()
		}
	}

	if actor == "" {
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

	switch r.GameType {
	case models.GameTypePoker:
		r.handlePokerTimeout()
	case models.GameTypeWerewolf:
		r.handleWerewolfTimeout()
	}

	// Check if game ended due to timeout action
	gameOver := false
	switch r.GameType {
	case models.GameTypePoker:
		gameOver = r.Poker != nil && r.Poker.Finished
	case models.GameTypeWerewolf:
		gameOver = r.Werewolf != nil && r.Werewolf.IsGameOver()
	}

	if gameOver {
		if r.OnGameOver != nil {
			go r.OnGameOver(r.GameID, r)
		}
	} else {
		// Reset timer for next actor
		r.ResetTurnTimer()
	}
}

func (r *Room) handlePokerTimeout() {
	if r.Poker == nil || r.Poker.Finished {
		return
	}

	actor := r.Poker.CurrentActor()
	if actor == "" {
		return
	}

	// Default action: check if possible, otherwise fold
	action := poker.Action{Type: poker.ActionFold}
	state := r.Poker.GetGameState(actor)
	for _, opt := range state.ValidActions {
		if opt.Type == poker.ActionCheck {
			action = poker.Action{Type: poker.ActionCheck}
			break
		}
	}

	slog.Info("turn timeout", "game_id", r.GameID, "player", actor, "default_action", action.Type)

	_, err := r.submitPokerAction(actor, mustJSON(action))
	if err != nil {
		slog.Error("timeout action failed", "game_id", r.GameID, "error", err)
	}
}

func (r *Room) handleWerewolfTimeout() {
	if r.Werewolf == nil || r.Werewolf.IsGameOver() {
		return
	}

	actor := r.Werewolf.CurrentActor()
	if actor == "" {
		return
	}

	// Default action depends on phase
	var action werewolf.Action
	switch r.Werewolf.Phase {
	case werewolf.PhaseNight:
		// Skip night action (wolf: random target, seer: random target)
		targets := r.Werewolf.GetValidTargets(actor)
		if len(targets) > 0 {
			player := r.Werewolf.FindPlayer(actor)
			if player != nil {
				switch player.Role {
				case werewolf.RoleWerewolf:
					action = werewolf.Action{Type: werewolf.ActionKill, TargetID: targets[0]}
				case werewolf.RoleSeer:
					action = werewolf.Action{Type: werewolf.ActionInvestigate, TargetID: targets[0]}
				}
			}
		}
	case werewolf.PhaseDay:
		action = werewolf.Action{Type: werewolf.ActionSpeak, Message: "..."}
	case werewolf.PhaseVote:
		action = werewolf.Action{Type: werewolf.ActionSkipVote}
	default:
		return
	}

	slog.Info("turn timeout", "game_id", r.GameID, "player", actor, "default_action", action.Type)

	_, err := r.submitWerewolfAction(actor, mustJSON(action))
	if err != nil {
		slog.Error("timeout action failed", "game_id", r.GameID, "error", err)
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
