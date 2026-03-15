package nats

import (
	"encoding/json"
	"fmt"
)

// --- Poker NATS subjects ---

const SubjectPokerRoomCreate = "poker.room.create"
const SubjectPokerRoomList = "poker.room.list"

func SubjectPokerRoomAction(roomID string) string {
	return fmt.Sprintf("poker.room.%s.action", roomID)
}

func SubjectPokerRoomState(roomID string) string {
	return fmt.Sprintf("poker.room.%s.state", roomID)
}

func SubjectPokerRoomSpectate(roomID string) string {
	return fmt.Sprintf("poker.room.%s.spectate", roomID)
}

func SubjectPokerRoomCleanup(roomID string) string {
	return fmt.Sprintf("poker.room.%s.cleanup", roomID)
}

func SubjectPokerEvent(roomID string) string {
	return fmt.Sprintf("poker.event.%s", roomID)
}

func SubjectPokerState(roomID, agentID string) string {
	return fmt.Sprintf("poker.state.%s.%s", roomID, agentID)
}

func SubjectPokerSpectate(roomID string) string {
	return fmt.Sprintf("poker.spectate.%s", roomID)
}

func SubjectPokerGameOver(roomID string) string {
	return fmt.Sprintf("poker.gameover.%s", roomID)
}

// --- Request/Response message types ---

// CreateRoomRequest is sent by api-gateway to create a poker room.
type CreateRoomRequest struct {
	GameID        string            `json:"game_id"`
	PlayerIDs     []string          `json:"player_ids"`
	PlayerNames   map[string]string `json:"player_names,omitempty"`   // id → display name
	PlayerAvatars map[string]string `json:"player_avatars,omitempty"` // id → avatar URL
	Seed          int64             `json:"seed"`
	EntryFee      int               `json:"entry_fee"`
}

// CreateRoomResponse is returned by poker-engine.
type CreateRoomResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ActionRequest is sent by api-gateway to submit a player action.
type ActionRequest struct {
	AgentID string          `json:"agent_id"`
	Action  json.RawMessage `json:"action"`
}

// ActionResponse is returned by poker-engine after processing an action.
type ActionResponse struct {
	Success   bool            `json:"success"`
	Error     string          `json:"error,omitempty"`
	Events    json.RawMessage `json:"events,omitempty"`
	GameOver  bool            `json:"game_over"`
	NextActor string          `json:"next_actor,omitempty"`
}

// StateRequest asks for a player's personalized game state.
type StateRequest struct {
	AgentID string `json:"agent_id"`
}

// StateResponse returns the game state.
type StateResponse struct {
	Success bool            `json:"success"`
	Error   string          `json:"error,omitempty"`
	State   json.RawMessage `json:"state,omitempty"`
}

// ListRoomsResponse returns all active poker rooms.
type ListRoomsResponse struct {
	Games []LiveGameInfo `json:"games"`
}

// LiveGameInfo is summary info about a live poker game.
type LiveGameInfo struct {
	GameID      string `json:"game_id"`
	PlayerCount int    `json:"player_count"`
	Phase       string `json:"phase"`
	HandNum     int    `json:"hand_num"`
}

// SubjectPokerTurnNotify returns the subject for turn notifications.
// Published after each action to notify the next actor.
func SubjectPokerTurnNotify(roomID string) string {
	return fmt.Sprintf("poker.turn.%s", roomID)
}

// TurnNotifyEvent is published to notify an agent it's their turn.
type TurnNotifyEvent struct {
	GameID  string `json:"game_id"`
	AgentID string `json:"agent_id"`
}

// GameOverEvent is published when a poker game finishes.
type GameOverEvent struct {
	GameID            string          `json:"game_id"`
	Rankings          json.RawMessage `json:"rankings"`
	AccumulatedEvents json.RawMessage `json:"accumulated_events"`
	EntryFee          int             `json:"entry_fee"`
}
