package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

const (
	writeTimeout = 10 * time.Second
	pingInterval = 15 * time.Second
	pongWait     = 45 * time.Second // 3 missed pings
	sendBufSize  = 64
)

// Conn wraps a WebSocket connection with metadata and buffered writing.
type Conn struct {
	ws      *websocket.Conn
	AgentID string // empty for spectators
	GameID  string
	send    chan []byte
	once    sync.Once
	done    chan struct{}
}

// NewConn creates a new wrapped connection.
func NewConn(ws *websocket.Conn, agentID, gameID string) *Conn {
	return &Conn{
		ws:      ws,
		AgentID: agentID,
		GameID:  gameID,
		send:    make(chan []byte, sendBufSize),
		done:    make(chan struct{}),
	}
}

// Send queues a message for sending. Non-blocking; drops if buffer full.
func (c *Conn) Send(msg []byte) {
	select {
	case c.send <- msg:
	default:
		slog.Warn("ws send buffer full, dropping message", "agent_id", c.AgentID)
	}
}

// Close closes the connection.
func (c *Conn) Close() {
	c.once.Do(func() {
		close(c.done)
		c.ws.Close(websocket.StatusNormalClosure, "")
	})
}

// WritePump drains the send channel and writes to the WebSocket.
func (c *Conn) WritePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := c.ws.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				slog.Debug("ws write error", "agent_id", c.AgentID, "error", err)
				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := c.ws.Ping(ctx)
			cancel()
			if err != nil {
				slog.Debug("ws ping failed", "agent_id", c.AgentID, "error", err)
				return
			}

		case <-c.done:
			return
		}
	}
}

// ReadPump reads messages from the WebSocket and passes them to the handler.
func (c *Conn) ReadPump(handler func(msg []byte)) {
	defer c.Close()

	for {
		_, data, err := c.ws.Read(context.Background())
		if err != nil {
			// Normal close or network error
			return
		}
		handler(data)
	}
}

// --- Wire protocol messages ---

// IncomingMsg is the message format agents send via WebSocket.
type IncomingMsg struct {
	Type   string          `json:"type"`   // "action", "ping"
	Action json.RawMessage `json:"action,omitempty"`
}

// OutgoingMsg is the message format sent to clients.
type OutgoingMsg struct {
	Type    string      `json:"type"` // "state", "event", "error", "match_found", "pong"
	GameID  string      `json:"game_id,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}
