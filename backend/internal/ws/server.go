package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/moltgame/backend/internal/auth"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/nats-io/nats.go"
	"nhooyr.io/websocket"
)

// Server handles WebSocket connections.
// All game state is fetched from poker-engine via NATS (no direct room.Manager dependency).
type Server struct {
	hub       *Hub
	nats      *natsClient.Client
	agentRepo auth.AgentFinder
}

// NewServer creates a new WebSocket server.
func NewServer(hub *Hub, nc *natsClient.Client, agentRepo auth.AgentFinder) *Server {
	return &Server{
		hub:       hub,
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

	// Fetch initial state from poker-engine via NATS to verify game exists and agent is in it
	var stateResp natsClient.StateResponse
	err = s.nats.RequestJSON(natsClient.SubjectPokerRoomState(gameID), natsClient.StateRequest{
		AgentID: agentID,
	}, &stateResp, 3*1e9) // 3 seconds
	if err != nil || !stateResp.Success {
		http.Error(w, "game not found or not in game", http.StatusNotFound)
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

	// Send current game state immediately
	s.sendMsg(conn, OutgoingMsg{Type: "state", GameID: gameID, Payload: json.RawMessage(stateResp.State)})

	// Start write pump in background
	go conn.WritePump()

	// Read pump (blocks until connection closes)
	conn.ReadPump(func(data []byte) {
		s.handleAgentMessage(conn, data)
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

	// Fetch spectator state from poker-engine via NATS
	var stateResp natsClient.StateResponse
	err := s.nats.RequestJSON(natsClient.SubjectPokerRoomSpectate(gameID), struct{}{}, &stateResp, 3*1e9)
	if err != nil || !stateResp.Success {
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
	s.sendMsg(conn, OutgoingMsg{Type: "state", GameID: gameID, Payload: json.RawMessage(stateResp.State)})

	go conn.WritePump()

	// Spectators don't send meaningful messages, but we still need to read to detect close
	conn.ReadPump(func(data []byte) {
		// Ignore spectator messages
	})
}

// --- Internal helpers ---

func (s *Server) handleAgentMessage(conn *Conn, data []byte) {
	var msg IncomingMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		s.sendMsg(conn, OutgoingMsg{Type: "error", Error: "invalid message format"})
		return
	}

	switch msg.Type {
	case "action":
		s.handleAction(conn, msg.Action)
	case "ping":
		s.sendMsg(conn, OutgoingMsg{Type: "pong"})
	default:
		s.sendMsg(conn, OutgoingMsg{Type: "error", Error: "unknown message type: " + msg.Type})
	}
}

func (s *Server) handleAction(conn *Conn, actionJSON json.RawMessage) {
	// Forward action to poker-engine via NATS
	var resp natsClient.ActionResponse
	err := s.nats.RequestJSON(natsClient.SubjectPokerRoomAction(conn.GameID), natsClient.ActionRequest{
		AgentID: conn.AgentID,
		Action:  actionJSON,
	}, &resp, 3*1e9)
	if err != nil {
		s.sendMsg(conn, OutgoingMsg{Type: "error", Error: "engine unavailable"})
		return
	}
	if !resp.Success {
		s.sendMsg(conn, OutgoingMsg{Type: "error", Error: resp.Error})
		return
	}

	// The poker-engine broadcasts events/state via NATS;
	// SubscribeNATSEvents picks those up and sends to WebSocket clients.
	// We also send the action response directly to the acting agent.
	s.sendMsg(conn, OutgoingMsg{Type: "action_result", GameID: conn.GameID, Payload: resp})
}

func (s *Server) sendMsg(conn *Conn, msg OutgoingMsg) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	conn.Send(data)
}

// SubscribeNATSEvents subscribes to NATS for game state broadcasts and routes them to WebSocket.
func (s *Server) SubscribeNATSEvents(ctx context.Context) error {
	if s.nats == nil {
		return nil
	}

	// Subscribe to per-agent state updates: poker.state.{roomID}.{agentID}
	_, err := s.nats.Subscribe("poker.state.>", func(msg *nats.Msg) {
		parts := splitSubject(msg.Subject)
		// poker.state.{roomID}.{agentID}
		if len(parts) < 4 {
			return
		}
		roomID := parts[2]
		agentID := parts[3]

		stateMsg, _ := json.Marshal(OutgoingMsg{
			Type:    "state",
			GameID:  roomID,
			Payload: json.RawMessage(msg.Data),
		})
		s.hub.SendToAgent(agentID, stateMsg)
	})
	if err != nil {
		return err
	}

	// Subscribe to spectator broadcasts: poker.spectate.{roomID}
	_, err = s.nats.Subscribe("poker.spectate.>", func(msg *nats.Msg) {
		parts := splitSubject(msg.Subject)
		if len(parts) < 3 {
			return
		}
		roomID := parts[2]

		specMsg, _ := json.Marshal(OutgoingMsg{
			Type:    "state",
			GameID:  roomID,
			Payload: json.RawMessage(msg.Data),
		})
		s.hub.BroadcastToSpectators(roomID, specMsg)
	})
	if err != nil {
		return err
	}

	// Subscribe to game events: poker.event.{roomID}
	_, err = s.nats.Subscribe("poker.event.>", func(msg *nats.Msg) {
		parts := splitSubject(msg.Subject)
		if len(parts) < 3 {
			return
		}
		roomID := parts[2]

		eventMsg, _ := json.Marshal(OutgoingMsg{
			Type:    "event",
			GameID:  roomID,
			Payload: json.RawMessage(msg.Data),
		})
		s.hub.BroadcastToRoom(roomID, eventMsg)
	})
	if err != nil {
		return err
	}

	// Subscribe to match_found events
	_, err = s.nats.Subscribe(natsClient.SubjectMatchmaking("poker"), func(msg *nats.Msg) {
		var matchMsg natsClient.MatchFoundMsg
		if err := json.Unmarshal(msg.Data, &matchMsg); err != nil {
			return
		}
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

	return nil
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
