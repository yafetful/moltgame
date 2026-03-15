package api

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/internal/chakra"
	gameRepo "github.com/moltgame/backend/internal/game"
	"github.com/moltgame/backend/internal/twitter"
	"github.com/moltgame/backend/pkg/httputil"
)

const (
	checkInAmount   = 100
	initialChakra   = 2000
	checkInCooldown = 4 * time.Hour
)

type OwnerHandler struct {
	agentRepo     *auth.AgentRepository
	ownerRepo     *auth.OwnerRepository
	chakraRepo    *chakra.Repository
	gameRepo      *gameRepo.Repository
	twitterClient *twitter.Client
}

func NewOwnerHandler(agentRepo *auth.AgentRepository, ownerRepo *auth.OwnerRepository, chakraRepo *chakra.Repository, gr *gameRepo.Repository, tc *twitter.Client) *OwnerHandler {
	return &OwnerHandler{
		agentRepo:     agentRepo,
		ownerRepo:     ownerRepo,
		chakraRepo:    chakraRepo,
		gameRepo:      gr,
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

	// Enforce 24h cooldown via owner_accounts.last_check_in
	owner, err := h.ownerRepo.GetOwner(r.Context(), twitterID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "owner_error", "Failed to fetch owner")
		return
	}
	nextCheckIn := time.Time{}
	if owner.LastCheckIn != nil {
		nextCheckIn = owner.LastCheckIn.Add(checkInCooldown)
		if time.Now().Before(nextCheckIn) {
			httputil.JSON(w, http.StatusTooManyRequests, map[string]any{
				"code":          "already_checked_in",
				"error":         "Already checked in today",
				"next_check_in": nextCheckIn,
			})
			return
		}
	}

	if err := h.chakraRepo.CheckIn(r.Context(), agentID, checkInAmount); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "checkin_error", "Failed to process check-in")
		return
	}

	if err := h.ownerRepo.UpdateCheckIn(r.Context(), twitterID); err != nil {
		slog.Warn("failed to update last_check_in", "error", err, "twitter_id", twitterID)
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"message":       "Check-in successful",
		"chakra_added":  checkInAmount,
		"next_check_in": time.Now().Add(checkInCooldown),
	})
}

const (
	maxAvatarSize = 2 << 20 // 2 MB
	uploadsDir    = "./uploads/avatars"
)

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// UploadAvatar handles POST /api/v1/owner/agents/{id}/avatar
// Accepts multipart/form-data with field "avatar" (max 2MB, JPEG/PNG/WebP/GIF).
func (h *OwnerHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
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

	// Limit total request body to avoid OOM
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize+512)
	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		httputil.Error(w, http.StatusBadRequest, "file_too_large", "Avatar must be under 2 MB")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "missing_file", "No avatar file provided")
		return
	}
	defer file.Close()

	// Detect content type from first 512 bytes
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	// Go's DetectContentType doesn't recognise WebP; check magic bytes manually
	// WebP: "RIFF" at [0:4] and "WEBP" at [8:12]
	if len(buf) >= 12 && string(buf[0:4]) == "RIFF" && string(buf[8:12]) == "WEBP" {
		contentType = "image/webp"
	}
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		// Fallback: check declared content type in multipart header
		declared := header.Header.Get("Content-Type")
		ext, ok = allowedImageTypes[strings.Split(declared, ";")[0]]
		if !ok {
			httputil.Error(w, http.StatusBadRequest, "invalid_type", "Only JPEG, PNG, WebP, or GIF allowed")
			return
		}
	}

	// Ensure uploads directory exists
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "storage_error", "Failed to create upload directory")
		return
	}

	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	destPath := filepath.Join(uploadsDir, filename)

	dest, err := os.Create(destPath)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "storage_error", "Failed to save avatar")
		return
	}
	defer dest.Close()

	// Write first 512 bytes already read, then the rest
	if _, err := dest.Write(buf[:n]); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "storage_error", "Failed to write avatar")
		return
	}
	if _, err := io.Copy(dest, file); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "storage_error", "Failed to write avatar")
		return
	}

	avatarURL := "/uploads/avatars/" + filename
	if err := h.agentRepo.UpdateAgentProfile(r.Context(), agentID, agent.Description, avatarURL); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "update_error", "Failed to update agent avatar")
		return
	}

	slog.Info("avatar uploaded", "agent_id", agentID, "file", filename)
	httputil.JSON(w, http.StatusOK, map[string]string{"avatar_url": avatarURL})
}

// GetMyAgentHistory handles GET /api/v1/owner/agent/history
// Returns the game history for the owner's bound agent.
func (h *OwnerHandler) GetMyAgentHistory(w http.ResponseWriter, r *http.Request) {
	twitterID := auth.GetOwnerID(r.Context())
	if twitterID == "" {
		httputil.Error(w, http.StatusUnauthorized, "no_auth", "Owner authentication required")
		return
	}

	agents, err := h.agentRepo.GetAgentsByOwner(r.Context(), twitterID)
	if err != nil || len(agents) == 0 {
		httputil.JSON(w, http.StatusOK, []any{})
		return
	}

	history, err := h.gameRepo.GetAgentHistory(r.Context(), agents[0].ID, 20)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "query_failed", "Failed to fetch history")
		return
	}
	if history == nil {
		history = []gameRepo.AgentGameHistory{}
	}
	httputil.JSON(w, http.StatusOK, history)
}
