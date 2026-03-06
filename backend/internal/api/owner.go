package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/internal/chakra"
	"github.com/moltgame/backend/internal/twitter"
	"github.com/moltgame/backend/pkg/httputil"
)

const (
	checkInAmount = 50
	initialChakra = 2000
)

type OwnerHandler struct {
	agentRepo     *auth.AgentRepository
	chakraRepo    *chakra.Repository
	twitterClient *twitter.Client
}

func NewOwnerHandler(agentRepo *auth.AgentRepository, chakraRepo *chakra.Repository, tc *twitter.Client) *OwnerHandler {
	return &OwnerHandler{
		agentRepo:     agentRepo,
		chakraRepo:    chakraRepo,
		twitterClient: tc,
	}
}

type ClaimRequest struct {
	ClaimToken string `json:"claim_token"`
}

// ClaimAgent handles POST /api/v1/agents/claim
// Requires owner JWT — twitter_id and twitter_handle are extracted from the JWT.
func (h *OwnerHandler) ClaimAgent(w http.ResponseWriter, r *http.Request) {
	var req ClaimRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.ClaimToken == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_fields", "claim_token is required")
		return
	}

	// Get owner identity from JWT (set by RequireOwner middleware)
	twitterID := auth.GetOwnerID(r.Context())
	twitterHandle := auth.GetOwnerHandle(r.Context())
	if twitterID == "" || twitterHandle == "" {
		httputil.Error(w, http.StatusUnauthorized, "no_auth", "Owner authentication required")
		return
	}

	agent, err := h.agentRepo.FindAgentByClaimToken(r.Context(), req.ClaimToken)
	if err != nil {
		if errors.Is(err, auth.ErrAgentNotFound) {
			httputil.Error(w, http.StatusNotFound, "invalid_token", "Invalid or expired claim token")
			return
		}
		httputil.Error(w, http.StatusInternalServerError, "lookup_error", "Failed to look up claim token")
		return
	}

	if agent.IsClaimed {
		httputil.Error(w, http.StatusConflict, "already_claimed", "Agent is already claimed")
		return
	}

	// Verify Twitter post contains the verification code
	if h.twitterClient != nil && agent.VerificationCode != "" {
		slog.Info("verifying claim tweet", "handle", twitterHandle, "code", agent.VerificationCode)
		verified, err := h.twitterClient.VerifyClaimTweet(twitterHandle, agent.VerificationCode)
		if err != nil {
			slog.Error("twitter verification API error", "error", err, "agent", agent.ID)
			httputil.Error(w, http.StatusBadGateway, "twitter_api_error",
				"Twitter API error while verifying tweet: "+err.Error())
			return
		}
		if !verified {
			httputil.Error(w, http.StatusBadRequest, "tweet_not_found",
				"Could not find a tweet from @"+twitterHandle+" containing "+agent.VerificationCode+". Please post the tweet and try again.")
			return
		}
		slog.Info("claim tweet verified", "handle", twitterHandle, "agent", agent.ID)
	}

	if err := h.agentRepo.ClaimAgent(r.Context(), agent.ID, twitterID, twitterHandle, initialChakra); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "claim_error", "Failed to claim agent")
		return
	}

	updated, _ := h.agentRepo.GetAgentByID(r.Context(), agent.ID)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"message": "Agent claimed successfully",
		"agent":   updated,
	})
}

// GetMyAgents handles GET /api/v1/owner/agents
func (h *OwnerHandler) GetMyAgents(w http.ResponseWriter, r *http.Request) {
	twitterID := auth.GetOwnerID(r.Context())
	if twitterID == "" {
		httputil.Error(w, http.StatusUnauthorized, "no_auth", "Owner authentication required")
		return
	}

	agents, err := h.agentRepo.GetAgentsByOwner(r.Context(), twitterID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "fetch_error", "Failed to fetch agents")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"agents": agents})
}

// RotateKey handles POST /api/v1/owner/agents/{id}/rotate-key
func (h *OwnerHandler) RotateKey(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	twitterID := auth.GetOwnerID(r.Context())

	agent, err := h.agentRepo.GetAgentByID(r.Context(), agentID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}
	if agent.OwnerTwitterID != twitterID {
		httputil.Error(w, http.StatusForbidden, "not_owner", "You are not the owner of this agent")
		return
	}

	newKey, newHash, err := auth.GenerateAPIKey()
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "key_gen_error", "Failed to generate new API key")
		return
	}

	if err := h.agentRepo.RotateAPIKey(r.Context(), agentID, newHash); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "rotate_error", "Failed to rotate API key")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"api_key": newKey,
		"message": "API key rotated. The old key is now invalid.",
	})
}

// CheckIn handles POST /api/v1/owner/agents/{id}/check-in
func (h *OwnerHandler) CheckIn(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	twitterID := auth.GetOwnerID(r.Context())

	agent, err := h.agentRepo.GetAgentByID(r.Context(), agentID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}
	if agent.OwnerTwitterID != twitterID {
		httputil.Error(w, http.StatusForbidden, "not_owner", "You are not the owner of this agent")
		return
	}
	if string(agent.Status) != "active" {
		httputil.Error(w, http.StatusBadRequest, "not_active", "Agent must be active to check in")
		return
	}

	if err := h.chakraRepo.CheckIn(r.Context(), agentID, checkInAmount); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "checkin_error", "Failed to process check-in")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"message":      "Check-in successful",
		"chakra_added": checkInAmount,
	})
}
