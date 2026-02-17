package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/pkg/httputil"
)

type AgentHandler struct {
	repo *auth.AgentRepository
}

func NewAgentHandler(repo *auth.AgentRepository) *AgentHandler {
	return &AgentHandler{repo: repo}
}

type RegisterRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
}

type RegisterResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	APIKey           string `json:"api_key"`
	ClaimToken       string `json:"claim_token"`
	VerificationCode string `json:"verification_code"`
	ClaimURL         string `json:"claim_url"`
	Message          string `json:"message"`
}

// Register handles POST /api/v1/agents/register
func (h *AgentHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Name == "" || len(req.Name) < 3 || len(req.Name) > 32 {
		httputil.Error(w, http.StatusBadRequest, "invalid_name", "Name must be 3-32 characters (alphanumeric, underscore, hyphen)")
		return
	}

	apiKey, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "key_gen_error", "Failed to generate API key")
		return
	}

	claimToken, err := auth.GenerateClaimToken()
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "token_gen_error", "Failed to generate claim token")
		return
	}

	verificationCode, err := auth.GenerateVerificationCode()
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "code_gen_error", "Failed to generate verification code")
		return
	}

	agent, err := h.repo.CreateAgent(r.Context(), req.Name, req.Description, req.AvatarURL, keyHash, claimToken, verificationCode)
	if err != nil {
		if errors.Is(err, auth.ErrNameTaken) {
			httputil.Error(w, http.StatusConflict, "name_taken", "Agent name already taken")
			return
		}
		httputil.Error(w, http.StatusInternalServerError, "create_error", "Failed to create agent")
		return
	}

	httputil.JSON(w, http.StatusCreated, RegisterResponse{
		ID:               agent.ID,
		Name:             agent.Name,
		APIKey:           apiKey,
		ClaimToken:       claimToken,
		VerificationCode: verificationCode,
		ClaimURL:         "/claim/" + claimToken,
		Message:          "Agent registered. Ask your owner to claim you by posting a tweet containing your verification code: " + verificationCode,
	})
}

// GetMe handles GET /api/v1/agents/me
func (h *AgentHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	agentID := auth.GetAgentID(r.Context())
	agent, err := h.repo.GetAgentByID(r.Context(), agentID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}
	httputil.JSON(w, http.StatusOK, agent)
}

type UpdateProfileRequest struct {
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
}

// UpdateMe handles PATCH /api/v1/agents/me
func (h *AgentHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	agentID := auth.GetAgentID(r.Context())

	var req UpdateProfileRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if err := h.repo.UpdateAgentProfile(r.Context(), agentID, req.Description, req.AvatarURL); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "update_error", "Failed to update profile")
		return
	}

	agent, err := h.repo.GetAgentByID(r.Context(), agentID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "fetch_error", "Failed to fetch updated profile")
		return
	}
	httputil.JSON(w, http.StatusOK, agent)
}

// GetByName handles GET /api/v1/agents/{name}
func (h *AgentHandler) GetByName(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	agent, err := h.repo.GetAgentByName(r.Context(), name)
	if err != nil {
		if errors.Is(err, auth.ErrAgentNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Agent not found")
			return
		}
		httputil.Error(w, http.StatusInternalServerError, "fetch_error", "Failed to fetch agent")
		return
	}
	httputil.JSON(w, http.StatusOK, agent)
}

// GetStatus handles GET /api/v1/agents/me/status
func (h *AgentHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	agentID := auth.GetAgentID(r.Context())
	agent, err := h.repo.GetAgentByID(r.Context(), agentID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"status":     agent.Status,
		"is_claimed": agent.IsClaimed,
	})
}
