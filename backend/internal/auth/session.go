package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidSession = errors.New("invalid or expired session")

const ownerIDKey contextKey = "owner_twitter_id"
const ownerHandleKey contextKey = "owner_twitter_handle"

// SessionManager handles JWT-based owner sessions.
type SessionManager struct {
	secret []byte
}

func NewSessionManager(secret string) *SessionManager {
	return &SessionManager{secret: []byte(secret)}
}

type OwnerClaims struct {
	TwitterID     string `json:"twitter_id"`
	TwitterHandle string `json:"twitter_handle"`
	jwt.RegisteredClaims
}

// CreateToken generates a JWT for an authenticated owner.
func (sm *SessionManager) CreateToken(twitterID, twitterHandle string) (string, error) {
	claims := OwnerClaims{
		TwitterID:     twitterID,
		TwitterHandle: twitterHandle,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "moltgame",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(sm.secret)
}

// ValidateToken parses and validates a JWT, returning the claims.
func (sm *SessionManager) ValidateToken(tokenString string) (*OwnerClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &OwnerClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return sm.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidSession
	}

	claims, ok := token.Claims.(*OwnerClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidSession
	}
	return claims, nil
}

// RequireOwner is a middleware that extracts and validates owner JWT from Authorization header.
func RequireOwner(sm *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"code":"no_auth","error":"Owner authentication required"}`, http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			// Skip if it's an agent API key (starts with moltgame_sk_)
			if strings.HasPrefix(tokenStr, APIKeyPrefix) {
				http.Error(w, `{"code":"wrong_auth","error":"Use owner token, not agent API key"}`, http.StatusUnauthorized)
				return
			}

			claims, err := sm.ValidateToken(tokenStr)
			if err != nil {
				http.Error(w, `{"code":"invalid_token","error":"Invalid or expired owner token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ownerIDKey, claims.TwitterID)
			ctx = context.WithValue(ctx, ownerHandleKey, claims.TwitterHandle)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetOwnerID returns the owner's Twitter ID from context.
func GetOwnerID(ctx context.Context) string {
	if v, ok := ctx.Value(ownerIDKey).(string); ok {
		return v
	}
	return ""
}

// GetOwnerHandle returns the owner's Twitter handle from context.
func GetOwnerHandle(ctx context.Context) string {
	if v, ok := ctx.Value(ownerHandleKey).(string); ok {
		return v
	}
	return ""
}
