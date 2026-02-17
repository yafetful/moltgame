package ws

import (
	"log/slog"
	"sync"
)

// Hub manages all WebSocket connections, organized by game room.
type Hub struct {
	mu sync.RWMutex

	// gameID → set of connections
	rooms map[string]map[*Conn]struct{}

	// agentID → connection (1 active connection per agent)
	agents map[string]*Conn

	// spectators per game
	spectators map[string]map[*Conn]struct{}
}

// NewHub creates a new connection hub.
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Conn]struct{}),
		agents:     make(map[string]*Conn),
		spectators: make(map[string]map[*Conn]struct{}),
	}
}

// RegisterAgent registers an agent connection for a specific game.
func (h *Hub) RegisterAgent(gameID, agentID string, conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close previous connection if exists
	if old, ok := h.agents[agentID]; ok {
		old.Close()
	}

	h.agents[agentID] = conn

	if h.rooms[gameID] == nil {
		h.rooms[gameID] = make(map[*Conn]struct{})
	}
	h.rooms[gameID][conn] = struct{}{}

	slog.Info("agent connected", "agent_id", agentID, "game_id", gameID)
}

// RegisterSpectator registers a spectator connection for a game.
func (h *Hub) RegisterSpectator(gameID string, conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.spectators[gameID] == nil {
		h.spectators[gameID] = make(map[*Conn]struct{})
	}
	h.spectators[gameID][conn] = struct{}{}

	slog.Info("spectator connected", "game_id", gameID)
}

// Unregister removes a connection from all tracking.
func (h *Hub) Unregister(conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from agents map
	if conn.AgentID != "" {
		if h.agents[conn.AgentID] == conn {
			delete(h.agents, conn.AgentID)
		}
	}

	// Remove from room
	if conn.GameID != "" {
		if room, ok := h.rooms[conn.GameID]; ok {
			delete(room, conn)
			if len(room) == 0 {
				delete(h.rooms, conn.GameID)
			}
		}
		if specs, ok := h.spectators[conn.GameID]; ok {
			delete(specs, conn)
			if len(specs) == 0 {
				delete(h.spectators, conn.GameID)
			}
		}
	}
}

// BroadcastToRoom sends a message to all connections in a game room (agents + spectators).
func (h *Hub) BroadcastToRoom(gameID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if room, ok := h.rooms[gameID]; ok {
		for conn := range room {
			conn.Send(msg)
		}
	}
	if specs, ok := h.spectators[gameID]; ok {
		for conn := range specs {
			conn.Send(msg)
		}
	}
}

// SendToAgent sends a message to a specific agent.
func (h *Hub) SendToAgent(agentID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conn, ok := h.agents[agentID]; ok {
		conn.Send(msg)
	}
}

// BroadcastToSpectators sends a message only to spectators of a game.
func (h *Hub) BroadcastToSpectators(gameID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if specs, ok := h.spectators[gameID]; ok {
		for conn := range specs {
			conn.Send(msg)
		}
	}
}

// SpectatorCount returns the number of spectators for a game.
func (h *Hub) SpectatorCount(gameID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.spectators[gameID])
}

// CleanupRoom removes all connections for a finished game.
func (h *Hub) CleanupRoom(gameID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[gameID]; ok {
		for conn := range room {
			conn.Close()
		}
		delete(h.rooms, gameID)
	}
	if specs, ok := h.spectators[gameID]; ok {
		for conn := range specs {
			conn.Close()
		}
		delete(h.spectators, gameID)
	}
}
