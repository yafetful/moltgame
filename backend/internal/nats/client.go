package nats

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// Client wraps the NATS connection with game-specific helpers.
type Client struct {
	conn *nats.Conn
}

// Connect establishes a NATS connection.
func Connect(addr string) (*Client, error) {
	nc, err := nats.Connect(addr,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("nats reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	slog.Info("nats connected", "addr", addr)
	return &Client{conn: nc}, nil
}

// Close drains and closes the connection.
func (c *Client) Close() {
	c.conn.Drain()
}

// Conn returns the underlying NATS connection.
func (c *Client) Conn() *nats.Conn {
	return c.conn
}

// --- Subject helpers ---

// Subject patterns:
//   game.{type}.{roomId}.action   — agent actions (engine consumes)
//   game.{type}.{roomId}.state    — state updates (agents consume)
//   game.{type}.{roomId}.spectate — spectator broadcast
//   game.{type}.{roomId}.event    — game events (Event Sourcing)
//   system.matchmaking.{type}     — matchmaking queue
//   system.agent.{agentId}.notify — personal notifications

func SubjectGameAction(gameType, roomID string) string {
	return fmt.Sprintf("game.%s.%s.action", gameType, roomID)
}

func SubjectGameState(gameType, roomID string) string {
	return fmt.Sprintf("game.%s.%s.state", gameType, roomID)
}

func SubjectGameSpectate(gameType, roomID string) string {
	return fmt.Sprintf("game.%s.%s.spectate", gameType, roomID)
}

func SubjectGameEvent(gameType, roomID string) string {
	return fmt.Sprintf("game.%s.%s.event", gameType, roomID)
}

func SubjectMatchmaking(gameType string) string {
	return fmt.Sprintf("system.matchmaking.%s", gameType)
}

func SubjectAgentNotify(agentID string) string {
	return fmt.Sprintf("system.agent.%s.notify", agentID)
}

// --- Publish helpers ---

// PublishJSON publishes a JSON-encoded message.
func (c *Client) PublishJSON(subject string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.conn.Publish(subject, payload)
}

// Subscribe subscribes to a subject with a handler.
func (c *Client) Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.conn.Subscribe(subject, handler)
}

// QueueSubscribe subscribes with a queue group for load balancing.
func (c *Client) QueueSubscribe(subject, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.conn.QueueSubscribe(subject, queue, handler)
}

// --- Message types ---

// GameStateMsg is broadcast when game state changes.
type GameStateMsg struct {
	GameID   string      `json:"game_id"`
	GameType string      `json:"game_type"`
	State    interface{} `json:"state"`
}

// GameActionMsg is an action submitted by an agent.
type GameActionMsg struct {
	GameID   string          `json:"game_id"`
	AgentID  string          `json:"agent_id"`
	Action   json.RawMessage `json:"action"`
}

// MatchFoundMsg is sent when a match is formed.
type MatchFoundMsg struct {
	GameID    string   `json:"game_id"`
	GameType  string   `json:"game_type"`
	PlayerIDs []string `json:"player_ids"`
}

// NotifyMsg is a personal notification for an agent.
type NotifyMsg struct {
	Type    string      `json:"type"` // "match_found", "game_start", "your_turn", etc.
	Payload interface{} `json:"payload"`
}
