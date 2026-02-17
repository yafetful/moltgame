package api

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/moltgame/backend/internal/auth"
	"github.com/moltgame/backend/internal/twitter"
	"github.com/moltgame/backend/pkg/httputil"
)

type AuthHandler struct {
	twitterClient *twitter.Client
	sessions      *auth.SessionManager

	// In-memory store for OAuth state → code_verifier mapping.
	// In production, use Redis with TTL.
	mu       sync.RWMutex
	pending  map[string]string // state → code_verifier
}

func NewAuthHandler(tc *twitter.Client, sm *auth.SessionManager) *AuthHandler {
	return &AuthHandler{
		twitterClient: tc,
		sessions:      sm,
		pending:       make(map[string]string),
	}
}

// StartTwitterAuth handles GET /api/v1/auth/twitter — returns the auth URL.
func (h *AuthHandler) StartTwitterAuth(w http.ResponseWriter, r *http.Request) {
	sess, err := h.twitterClient.StartAuth()
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "auth_error", "Failed to start Twitter auth")
		return
	}

	h.mu.Lock()
	h.pending[sess.State] = sess.CodeVerifier
	h.mu.Unlock()

	httputil.JSON(w, http.StatusOK, map[string]string{
		"auth_url": sess.AuthURL,
		"state":    sess.State,
	})
}

type twitterCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

// TwitterCallback handles POST /api/v1/auth/twitter/callback — exchanges code for session.
func (h *AuthHandler) TwitterCallback(w http.ResponseWriter, r *http.Request) {
	var req twitterCallbackRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Code == "" || req.State == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_fields", "code and state are required")
		return
	}

	// Look up and remove the code verifier
	h.mu.Lock()
	verifier, ok := h.pending[req.State]
	if ok {
		delete(h.pending, req.State)
	}
	h.mu.Unlock()

	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_state", "Invalid or expired OAuth state")
		return
	}

	// Exchange code for token
	tok, err := h.twitterClient.ExchangeCode(req.Code, verifier)
	if err != nil {
		slog.Error("twitter token exchange failed", "error", err)
		httputil.Error(w, http.StatusBadGateway, "token_exchange_error", "Failed to exchange Twitter code for token")
		return
	}

	// Get user info
	user, err := h.twitterClient.GetMe(tok.AccessToken)
	if err != nil {
		slog.Error("twitter get user failed", "error", err)
		httputil.Error(w, http.StatusBadGateway, "user_fetch_error", "Failed to fetch Twitter user info")
		return
	}

	// Create session JWT
	jwt, err := h.sessions.CreateToken(user.ID, user.Username)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "session_error", "Failed to create session")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"token":          jwt,
		"twitter_id":     user.ID,
		"twitter_handle": user.Username,
		"display_name":   user.Name,
	})
}
