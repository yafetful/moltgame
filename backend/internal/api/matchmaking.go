package api

import (
	"net/http"

	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/internal/matchmaking"
	"github.com/moltgame/backend/internal/models"
	"github.com/moltgame/backend/pkg/httputil"
)

// MatchmakingHandler handles matchmaking HTTP requests.
type MatchmakingHandler struct {
	matchSvc  *matchmaking.Service
	agentRepo *auth.AgentRepository
}

// NewMatchmakingHandler creates a new matchmaking handler.
func NewMatchmakingHandler(matchSvc *matchmaking.Service, agentRepo *auth.AgentRepository) *MatchmakingHandler {
	return &MatchmakingHandler{matchSvc: matchSvc, agentRepo: agentRepo}
}

type joinQueueRequest struct {
	GameType models.GameType `json:"game_type"`
}

// JoinQueue adds the authenticated agent to the matchmaking queue.
// POST /api/v1/matchmaking/join
func (h *MatchmakingHandler) JoinQueue(w http.ResponseWriter, r *http.Request) {
	agentID := auth.GetAgentID(r.Context())

	var req joinQueueRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	// Get agent's current rating
	agent, err := h.agentRepo.GetAgentByID(r.Context(), agentID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "agent_error", "Failed to get agent")
		return
	}

	if err := h.matchSvc.Join(req.GameType, agentID, agent.TrueSkillMu, agent.TrueSkillSigma); err != nil {
		httputil.Error(w, http.StatusConflict, "join_error", err.Error())
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]interface{}{
		"status":    "queued",
		"game_type": req.GameType,
		"queue":     h.matchSvc.QueueStatus(),
	})
}

type leaveQueueRequest struct {
	GameType models.GameType `json:"game_type"`
}

// LeaveQueue removes the authenticated agent from the matchmaking queue.
// DELETE /api/v1/matchmaking/leave
func (h *MatchmakingHandler) LeaveQueue(w http.ResponseWriter, r *http.Request) {
	agentID := auth.GetAgentID(r.Context())

	var req leaveQueueRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if err := h.matchSvc.Leave(req.GameType, agentID); err != nil {
		httputil.Error(w, http.StatusNotFound, "not_in_queue", err.Error())
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "left"})
}

// QueueStatus returns current queue sizes.
// GET /api/v1/matchmaking/status
func (h *MatchmakingHandler) QueueStatus(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, h.matchSvc.QueueStatus())
}
