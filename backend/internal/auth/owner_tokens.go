package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrTokenNotFound = errors.New("token not found or expired")

const (
	accessTokenTTL  = 2 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

// OwnerTokenStore persists OAuth tokens in Redis.
// access_token TTL=2h, refresh_token TTL=30d (one-time-use on Twitter's side).
type OwnerTokenStore struct {
	rdb *redis.Client
}

func NewOwnerTokenStore(rdb *redis.Client) *OwnerTokenStore {
	return &OwnerTokenStore{rdb: rdb}
}

func accessKey(twitterID string) string  { return "owner_access:" + twitterID }
func refreshKey(twitterID string) string { return "owner_refresh:" + twitterID }

// SaveTokens stores both tokens. If refreshToken is empty, only access is saved.
func (s *OwnerTokenStore) SaveTokens(ctx context.Context, twitterID, accessToken, refreshToken string) error {
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, accessKey(twitterID), accessToken, accessTokenTTL)
	if refreshToken != "" {
		pipe.Set(ctx, refreshKey(twitterID), refreshToken, refreshTokenTTL)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("save owner tokens: %w", err)
	}
	return nil
}

// GetAccessToken returns the access token or ErrTokenNotFound if expired/missing.
func (s *OwnerTokenStore) GetAccessToken(ctx context.Context, twitterID string) (string, error) {
	val, err := s.rdb.Get(ctx, accessKey(twitterID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	return val, nil
}

// GetRefreshToken returns the refresh token or ErrTokenNotFound if expired/missing.
func (s *OwnerTokenStore) GetRefreshToken(ctx context.Context, twitterID string) (string, error) {
	val, err := s.rdb.Get(ctx, refreshKey(twitterID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get refresh token: %w", err)
	}
	return val, nil
}

// DeleteTokens removes both tokens (e.g. on logout).
func (s *OwnerTokenStore) DeleteTokens(ctx context.Context, twitterID string) error {
	return s.rdb.Del(ctx, accessKey(twitterID), refreshKey(twitterID)).Err()
}
