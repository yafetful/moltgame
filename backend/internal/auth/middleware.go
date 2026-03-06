package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/moltgame/backend/pkg/httputil"
)

type contextKey string

const AgentIDKey contextKey = "agent_id"

// AgentFinder looks up an agent by API key hash.
type AgentFinder interface {
	FindAgentByKeyHash(ctx context.Context, keyHash string) (agentID string, err error)
}

// AgentStatusChecker checks whether an agent is active (claimed).
type AgentStatusChecker interface {
	IsAgentActive(ctx context.Context, agentID string) (bool, error)
}

// RequireAgent is middleware that authenticates agents via Bearer token.
func RequireAgent(finder AgentFinder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				httputil.Error(w, http.StatusUnauthorized, "missing_auth", "Authorization header required")
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				httputil.Error(w, http.StatusUnauthorized, "invalid_auth", "Bearer token required")
				return
			}

			apiKey := parts[1]
			if !strings.HasPrefix(apiKey, "moltgame_sk_") {
				httputil.Error(w, http.StatusUnauthorized, "invalid_key", "Invalid API key format")
				return
			}

			keyHash := HashAPIKey(apiKey)
			agentID, err := finder.FindAgentByKeyHash(r.Context(), keyHash)
			if err != nil {
				httputil.Error(w, http.StatusUnauthorized, "invalid_key", "Invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), AgentIDKey, agentID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireActiveAgent is middleware that checks if the authenticated agent is claimed/active.
// Must be used AFTER RequireAgent.
func RequireActiveAgent(checker AgentStatusChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agentID := GetAgentID(r.Context())
			if agentID == "" {
				httputil.Error(w, http.StatusUnauthorized, "missing_auth", "Agent authentication required")
				return
			}
			active, err := checker.IsAgentActive(r.Context(), agentID)
			if err != nil {
				httputil.Error(w, http.StatusInternalServerError, "status_check_error", "Failed to check agent status")
				return
			}
			if !active {
				httputil.Error(w, http.StatusForbidden, "not_active", "Agent must be claimed and active to perform this action. Visit the claim URL to activate your agent.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetAgentID extracts agent ID from the request context.
func GetAgentID(ctx context.Context) string {
	id, _ := ctx.Value(AgentIDKey).(string)
	return id
}

// HashAPIKey returns the SHA-256 hex hash of an API key.
func HashAPIKey(apiKey string) string {
	h := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(h[:])
}
