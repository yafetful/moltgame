package api

import (
	"errors"
	"hash/fnv"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/pkg/httputil"
)

// DefaultAvatars is the list of 24 pre-set animal avatar paths.
var DefaultAvatars = []string{
	"/avatars/01-fox.png", "/avatars/02-koala.png", "/avatars/03-owl.png",
	"/avatars/04-cat.png", "/avatars/05-bear.png", "/avatars/06-rabbit.png",
	"/avatars/07-wolf.png", "/avatars/08-raccoon.png", "/avatars/09-tiger.png",
	"/avatars/10-penguin.png", "/avatars/11-monkey.png", "/avatars/12-eagle.png",
	"/avatars/13-crocodile.png", "/avatars/14-deer.png", "/avatars/15-panda.png",
	"/avatars/16-lion.png", "/avatars/17-parrot.png", "/avatars/18-flamingo.png",
	"/avatars/19-hedgehog.png", "/avatars/20-red-panda.png", "/avatars/21-horse.png",
	"/avatars/22-elephant.png", "/avatars/23-chameleon.png", "/avatars/24-hamster.png",
}

// DefaultAvatarForName returns a deterministic avatar path based on the agent name.
func DefaultAvatarForName(name string) string {
	h := fnv.New32a()
	h.Write([]byte(name))
	return DefaultAvatars[int(h.Sum32())%len(DefaultAvatars)]
}

type AgentHandler struct {
	repo      *auth.AgentRepository
	skipClaim bool
}

func NewAgentHandler(repo *auth.AgentRepository, skipClaim bool) *AgentHandler {
	return &AgentHandler{repo: repo, skipClaim: skipClaim}
}

type RegisterRequest struct {
	Name        string `json:"name"`
	Model       string `json:"model"`
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

	// Assign default avatar if not provided
	if req.AvatarURL == "" {
		req.AvatarURL = DefaultAvatarForName(req.Name)
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

	agent, err := h.repo.CreateAgent(r.Context(), req.Name, req.Model, req.Description, req.AvatarURL, keyHash, claimToken, verificationCode)
	if err != nil {
		if errors.Is(err, auth.ErrNameTaken) {
			httputil.Error(w, http.StatusConflict, "name_taken", "Agent name already taken")
			return
		}
		httputil.Error(w, http.StatusInternalServerError, "create_error", "Failed to create agent")
		return
	}

	// Dev mode: auto-activate agent, grant Chakra, skip Twitter claim.
	// Uses ActivateAgent (not ClaimAgent) so owner_twitter_id stays empty
	// and the real dev can still bind later via /owner/bind/confirm.
	if h.skipClaim {
		if err := h.repo.ActivateAgent(r.Context(), agent.ID, 2000); err != nil {
			slog.Warn("skip-claim auto-activate failed", "agent_id", agent.ID, "error", err)
		} else {
			slog.Info("agent auto-activated (SKIP_CLAIM)", "agent_id", agent.ID, "name", req.Name)
		}
		httputil.JSON(w, http.StatusCreated, RegisterResponse{
			ID:               agent.ID,
			Name:             agent.Name,
			APIKey:           apiKey,
			VerificationCode: verificationCode,
			Message:          "Agent registered and auto-activated (dev mode). Save your API key! Verification code: " + verificationCode,
		})
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

// PublicStats returns aggregate platform stats (no auth required).
// GET /api/v1/stats
func (h *AgentHandler) PublicStats(w http.ResponseWriter, r *http.Request) {
	count, err := h.repo.CountAgents(r.Context())
	if err != nil {
		count = 0
	}
	httputil.JSON(w, http.StatusOK, map[string]int{
		"total_agents": count,
	})
}

// Leaderboard returns ranked agents with stats.
// GET /api/v1/leaderboard
func (h *AgentHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := h.repo.GetLeaderboard(r.Context())
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "query_failed", "Failed to fetch leaderboard")
		return
	}
	httputil.JSON(w, http.StatusOK, entries)
}
