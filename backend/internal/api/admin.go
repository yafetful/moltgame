package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/moltgame/backend/internal/aibot"
	"github.com/moltgame/backend/pkg/httputil"
)

// AdminHandler handles admin-only endpoints.
type AdminHandler struct {
	aiRunner *aibot.Runner
	password string
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(aiRunner *aibot.Runner, password string) *AdminHandler {
	return &AdminHandler{
		aiRunner: aiRunner,
		password: password,
	}
}

// StartAIGame triggers a new AI poker game.
// POST /api/v1/admin/start-ai-game
func (h *AdminHandler) StartAIGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if h.password == "" {
		httputil.Error(w, http.StatusServiceUnavailable, "not_configured", "Admin password not configured")
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.password)) != 1 {
		httputil.Error(w, http.StatusUnauthorized, "invalid_password", "Invalid password")
		return
	}

	if h.aiRunner == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "not_configured", "AI runner not configured")
		return
	}

	gameID, err := h.aiRunner.StartGame(r.Context())
	if err != nil {
		httputil.Error(w, http.StatusConflict, "start_failed", err.Error())
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{
		"game_id": gameID,
	})
}

// GetAIGameStatus returns the current AI game status.
// GET /api/v1/admin/ai-game-status
func (h *AdminHandler) GetAIGameStatus(w http.ResponseWriter, r *http.Request) {
	if h.aiRunner == nil {
		httputil.JSON(w, http.StatusOK, map[string]interface{}{
			"running": false,
		})
		return
	}

	running, gameID := h.aiRunner.IsRunning()
	resp := map[string]interface{}{
		"running": running,
	}
	if running {
		resp["game_id"] = gameID
	}
	httputil.JSON(w, http.StatusOK, resp)
}
