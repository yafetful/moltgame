package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/internal/twitter"
	"github.com/moltgame/backend/pkg/httputil"
)

const bindBonusChakra = 2000

// tweetTemplate is the text posted on behalf of the dev when binding an agent.
// %s placeholders: agent name, verification code.
const tweetTemplate = "My AI agent %s is now competing on @MoltGame! 🤖⚡\nVerification: %s\nWatch live at game.0ai.ai #MoltGame #AIAgent"

type DevBindHandler struct {
	agentRepo     *auth.AgentRepository
	ownerRepo     *auth.OwnerRepository
	tokenStore    *auth.OwnerTokenStore
	twitterClient *twitter.Client
}

func NewDevBindHandler(
	agentRepo *auth.AgentRepository,
	ownerRepo *auth.OwnerRepository,
	tokenStore *auth.OwnerTokenStore,
	tc *twitter.Client,
) *DevBindHandler {
	return &DevBindHandler{
		agentRepo:     agentRepo,
		ownerRepo:     ownerRepo,
		tokenStore:    tokenStore,
		twitterClient: tc,
	}
}

// GetOwnerMe handles GET /api/v1/owner/me
// Returns owner profile and their bound agent if any.
func (h *DevBindHandler) GetOwnerMe(w http.ResponseWriter, r *http.Request) {
	twitterID := auth.GetOwnerID(r.Context())

	owner, err := h.ownerRepo.GetOwner(r.Context(), twitterID)
	if err != nil {
		if errors.Is(err, auth.ErrOwnerNotFound) {
			httputil.Error(w, http.StatusNotFound, "owner_not_found", "Owner account not found")
			return
		}
		httputil.Error(w, http.StatusInternalServerError, "fetch_error", "Failed to fetch owner")
		return
	}

	resp := map[string]any{"owner": owner}

	// Attach bound agent details if available
	if owner.BoundAgentID != nil {
		agent, err := h.agentRepo.GetAgentByID(r.Context(), *owner.BoundAgentID)
		if err == nil {
			resp["agent"] = agent
		}
	}

	httputil.JSON(w, http.StatusOK, resp)
}

type bindPreviewRequest struct {
	VerificationCode string `json:"verification_code"`
}

// BindPreview handles POST /api/v1/owner/bind/preview
// Validates the verification code and returns agent info + tweet template.
func (h *DevBindHandler) BindPreview(w http.ResponseWriter, r *http.Request) {
	twitterID := auth.GetOwnerID(r.Context())
	twitterHandle := auth.GetOwnerHandle(r.Context())

	var req bindPreviewRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if req.VerificationCode == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_code", "verification_code is required")
		return
	}

	// Check owner not already bound
	owner, err := h.ownerRepo.GetOwner(r.Context(), twitterID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "owner_error", "Failed to fetch owner")
		return
	}
	if owner.BoundAgentID != nil {
		httputil.Error(w, http.StatusConflict, "already_bound", "You already have a bound agent")
		return
	}

	// Look up agent by verification code
	agent, err := h.agentRepo.FindAgentByVerificationCode(r.Context(), req.VerificationCode)
	if err != nil {
		if errors.Is(err, auth.ErrAgentNotFound) {
			httputil.Error(w, http.StatusNotFound, "invalid_code", "Invalid or already-used verification code")
			return
		}
		httputil.Error(w, http.StatusInternalServerError, "lookup_error", "Failed to look up agent")
		return
	}

	// Agent must not already have an owner
	if agent.IsClaimed && agent.OwnerTwitterID != "" {
		httputil.Error(w, http.StatusConflict, "agent_taken", "This agent is already bound to another developer")
		return
	}

	tweet := fmt.Sprintf(tweetTemplate, agent.Name, req.VerificationCode)
	_ = twitterHandle // used implicitly via JWT; available if needed for logging

	httputil.JSON(w, http.StatusOK, map[string]any{
		"agent_id":       agent.ID,
		"agent_name":     agent.Name,
		"agent_avatar":   agent.AvatarURL,
		"agent_model":    agent.Model,
		"tweet_template": tweet,
	})
}

type bindConfirmRequest struct {
	VerificationCode string `json:"verification_code"`
}

// BindConfirm handles POST /api/v1/owner/bind/confirm
// Posts a tweet on behalf of the dev, then completes the binding atomically.
func (h *DevBindHandler) BindConfirm(w http.ResponseWriter, r *http.Request) {
	twitterID := auth.GetOwnerID(r.Context())
	twitterHandle := auth.GetOwnerHandle(r.Context())

	var req bindConfirmRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if req.VerificationCode == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_code", "verification_code is required")
		return
	}

	// Re-run all validations (idempotency guard)
	owner, err := h.ownerRepo.GetOwner(r.Context(), twitterID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "owner_error", "Failed to fetch owner")
		return
	}
	if owner.BoundAgentID != nil {
		httputil.Error(w, http.StatusConflict, "already_bound", "You already have a bound agent")
		return
	}

	agent, err := h.agentRepo.FindAgentByVerificationCode(r.Context(), req.VerificationCode)
	if err != nil {
		if errors.Is(err, auth.ErrAgentNotFound) {
			httputil.Error(w, http.StatusNotFound, "invalid_code", "Invalid or already-used verification code")
			return
		}
		httputil.Error(w, http.StatusInternalServerError, "lookup_error", "Failed to look up agent")
		return
	}
	if agent.IsClaimed && agent.OwnerTwitterID != "" {
		httputil.Error(w, http.StatusConflict, "agent_taken", "This agent is already bound to another developer")
		return
	}

	// Resolve a valid access token (refresh if needed)
	accessToken, err := h.resolveAccessToken(r, twitterID)
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "token_expired",
			"Twitter session expired. Please log in again to re-authorize.")
		return
	}

	// Post the tweet on behalf of the dev
	tweet := fmt.Sprintf(tweetTemplate, agent.Name, req.VerificationCode)
	tweetID, err := h.twitterClient.PostTweet(accessToken, tweet)
	if err != nil {
		slog.Error("post bind tweet failed", "error", err, "owner", twitterHandle, "agent", agent.Name)
		httputil.Error(w, http.StatusBadGateway, "tweet_failed",
			"Failed to post tweet: "+err.Error())
		return
	}
	slog.Info("bind tweet posted", "tweet_id", tweetID, "owner", twitterHandle, "agent", agent.Name)

	// Atomically bind owner to agent and credit bonus Chakra
	if err := h.ownerRepo.BindOwnerToAgent(r.Context(), twitterID, twitterHandle, agent.ID); err != nil {
		if errors.Is(err, auth.ErrAlreadyBound) {
			httputil.Error(w, http.StatusConflict, "already_bound", "Binding conflict — already bound")
			return
		}
		slog.Error("bind owner to agent failed", "error", err, "owner", twitterID, "agent", agent.ID)
		httputil.Error(w, http.StatusInternalServerError, "bind_error", "Failed to complete binding")
		return
	}

	// Return updated agent info
	updated, _ := h.agentRepo.GetAgentByID(r.Context(), agent.ID)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"message":        "Agent bound successfully",
		"tweet_id":       tweetID,
		"chakra_granted": bindBonusChakra,
		"agent":          updated,
	})
}

type updateAgentRequest struct {
	Model       string `json:"model"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
}

// UpdateMyAgent handles PATCH /api/v1/owner/agent
// Allows a bound dev to update their agent's model, description, and avatar.
func (h *DevBindHandler) UpdateMyAgent(w http.ResponseWriter, r *http.Request) {
	twitterID := auth.GetOwnerID(r.Context())

	owner, err := h.ownerRepo.GetOwner(r.Context(), twitterID)
	if err != nil || owner.BoundAgentID == nil {
		httputil.Error(w, http.StatusForbidden, "no_agent", "You have no bound agent to update")
		return
	}

	var req updateAgentRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if len(req.Description) > 500 {
		httputil.Error(w, http.StatusBadRequest, "invalid_description", "Description must be 500 characters or fewer")
		return
	}

	if err := h.agentRepo.UpdateAgentByOwner(r.Context(), *owner.BoundAgentID, req.Model, req.Description, req.AvatarURL); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "update_error", "Failed to update agent")
		return
	}

	updated, _ := h.agentRepo.GetAgentByID(r.Context(), *owner.BoundAgentID)
	httputil.JSON(w, http.StatusOK, updated)
}

// resolveAccessToken retrieves a valid access token, refreshing if necessary.
// Returns ErrTokenNotFound if both tokens are exhausted.
func (h *DevBindHandler) resolveAccessToken(r *http.Request, twitterID string) (string, error) {
	ctx := r.Context()

	// Try access token first
	accessToken, err := h.tokenStore.GetAccessToken(ctx, twitterID)
	if err == nil {
		return accessToken, nil
	}
	if !errors.Is(err, auth.ErrTokenNotFound) {
		return "", err
	}

	// Access token expired — try refresh
	refreshToken, err := h.tokenStore.GetRefreshToken(ctx, twitterID)
	if err != nil {
		return "", auth.ErrTokenNotFound
	}

	newTok, err := h.twitterClient.RefreshToken(refreshToken)
	if err != nil {
		slog.Warn("twitter token refresh failed", "error", err, "twitter_id", twitterID)
		return "", auth.ErrTokenNotFound
	}

	// Save the new tokens (refresh tokens are one-time-use)
	if saveErr := h.tokenStore.SaveTokens(ctx, twitterID, newTok.AccessToken, newTok.RefreshToken); saveErr != nil {
		slog.Warn("save refreshed tokens failed", "error", saveErr)
	}

	return newTok.AccessToken, nil
}
