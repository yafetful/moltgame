package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/moltgame/backend/internal/auth"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/moltgame/backend/internal/room"
	"github.com/nats-io/nats.go"
	"nhooyr.io/websocket"
)

// Server handles WebSocket connections.
type Server struct {
	hub       *Hub
	rooms     *room.Manager
	nats      *natsClient.Client
	agentRepo auth.AgentFinder
}

// NewServer creates a new WebSocket server.
func NewServer(hub *Hub, rooms *room.Manager, nc *natsClient.Client, agentRepo auth.AgentFinder) *Server {
	return &Server{
		hub:       hub,
		rooms:     rooms,
		nats:      nc,
		agentRepo: agentRepo,
	}
}

// HandleAgent handles WebSocket connections from agents (players).
// GET /ws/game/{gameID}?token=<api_key>
func (s *Server) HandleAgent(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	apiKey := r.URL.Query().Get("token")

	if gameID == "" || apiKey == "" {
		http.Error(w, "missing game_id or token", http.StatusBadRequest)
		return
	}

	// Authenticate
	keyHash := auth.HashAPIKey(apiKey)
	agentID, err := s.agentRepo.FindAgentByKeyHash(r.Context(), keyHash)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Verify agent is in this game
	rm := s.rooms.GetRoom(gameID)
	if rm == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}
	if !rm.HasPlayer(agentID) {
		http.Error(w, "not in this game", http.StatusForbidden)
		return
	}

	// Upgrade to WebSocket
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("ws accept error", "error", err)
		return
	}

	conn := NewConn(ws, agentID, gameID)
	s.hub.RegisterAgent(gameID, agentID, conn)
	defer s.hub.Unregister(conn)

	// Send current game state immediately (reconnect support)
	s.sendCurrentState(conn, rm, agentID)

	// Start write pump in background
	go conn.WritePump()

	// Read pump (blocks until connection closes)
	conn.ReadPump(func(data []byte) {
		s.handleAgentMessage(conn, rm, data)
	})
}

// HandleSpectator handles WebSocket connections from spectators.
// GET /ws/spectate/{gameID}
func (s *Server) HandleSpectator(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	if gameID == "" {
		http.Error(w, "missing game_id", http.StatusBadRequest)
		return
	}

	rm := s.rooms.GetRoom(gameID)
	if rm == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("ws accept error", "error", err)
		return
	}

	conn := NewConn(ws, "", gameID)
	s.hub.RegisterSpectator(gameID, conn)
	defer s.hub.Unregister(conn)

	// Send current spectator state
	state, err := rm.GetSpectatorState()
	if err == nil {
		s.sendMsg(conn, OutgoingMsg{Type: "state", GameID: gameID, Payload: state})
	}

	go conn.WritePump()

	// Spectators don't send meaningful messages, but we still need to read to detect close
	conn.ReadPump(func(data []byte) {
		// Ignore spectator messages
	})
}

// --- Internal helpers ---

func (s *Server) sendCurrentState(conn *Conn, rm *room.Room, agentID string) {
	state, err := rm.GetState(agentID)
	if err != nil {
		s.sendMsg(conn, OutgoingMsg{Type: "error", Error: err.Error()})
		return
	}
	s.sendMsg(conn, OutgoingMsg{Type: "state", GameID: rm.GameID, Payload: state})
}

func (s *Server) handleAgentMessage(conn *Conn, rm *room.Room, data []byte) {
	var msg IncomingMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		s.sendMsg(conn, OutgoingMsg{Type: "error", Error: "invalid message format"})
		return
	}

	switch msg.Type {
	case "action":
		s.handleAction(conn, rm, msg.Action)
	case "ping":
		s.sendMsg(conn, OutgoingMsg{Type: "pong"})
	default:
		s.sendMsg(conn, OutgoingMsg{Type: "error", Error: "unknown message type: " + msg.Type})
	}
}

func (s *Server) handleAction(conn *Conn, rm *room.Room, actionJSON json.RawMessage) {
	result, err := rm.SubmitAction(conn.AgentID, actionJSON)
	if err != nil {
		s.sendMsg(conn, OutgoingMsg{Type: "error", Error: err.Error()})
		return
	}

	// Broadcast event to all room connections
	eventMsg, _ := json.Marshal(OutgoingMsg{
		Type:    "event",
		GameID:  rm.GameID,
		Payload: result.Events,
	})
	s.hub.BroadcastToRoom(rm.GameID, eventMsg)

	// Send personalized state to each agent in the room
	for _, pid := range rm.PlayerIDs {
		state, err := rm.GetState(pid)
		if err != nil {
			continue
		}
		stateMsg, _ := json.Marshal(OutgoingMsg{
			Type:    "state",
			GameID:  rm.GameID,
			Payload: state,
		})
		s.hub.SendToAgent(pid, stateMsg)
	}

	// Broadcast spectator state
	specState, err := rm.GetSpectatorState()
	if err == nil {
		specMsg, _ := json.Marshal(OutgoingMsg{
			Type:    "state",
			GameID:  rm.GameID,
			Payload: specState,
		})
		s.hub.BroadcastToSpectators(rm.GameID, specMsg)
	}

	// Publish to NATS for external consumers
	if s.nats != nil {
		s.nats.PublishJSON(
			natsClient.SubjectGameEvent(string(rm.GameType), rm.GameID),
			result.Events,
		)
	}

	// Notify next actor
	if result.NextActor != "" {
		state, err := rm.GetState(result.NextActor)
		if err == nil {
			turnMsg, _ := json.Marshal(OutgoingMsg{
				Type:    "your_turn",
				GameID:  rm.GameID,
				Payload: state,
			})
			s.hub.SendToAgent(result.NextActor, turnMsg)
		}
	}
}

func (s *Server) sendMsg(conn *Conn, msg OutgoingMsg) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	conn.Send(data)
}

// SubscribeNATSEvents subscribes to NATS for match notifications and routes them to WebSocket.
func (s *Server) SubscribeNATSEvents(ctx context.Context) error {
	if s.nats == nil {
		return nil
	}

	// Subscribe to match_found events for all game types
	for _, gameType := range []string{"poker", "werewolf"} {
		subject := natsClient.SubjectMatchmaking(gameType)
		_, err := s.nats.Subscribe(subject, func(msg *nats.Msg) {
			var matchMsg natsClient.MatchFoundMsg
			if err := json.Unmarshal(msg.Data, &matchMsg); err != nil {
				return
			}
			// Notify each matched agent
			for _, agentID := range matchMsg.PlayerIDs {
				notifyData, _ := json.Marshal(OutgoingMsg{
					Type:    "match_found",
					GameID:  matchMsg.GameID,
					Payload: matchMsg,
				})
				s.hub.SendToAgent(agentID, notifyData)
			}
		})
		if err != nil {
			return err
		}
	}

	return nil
}
