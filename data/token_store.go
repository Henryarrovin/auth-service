package data

import (
	"auth-service/config"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenStore manages refresh token allowlisting in Redis.
// Only tokens present in Redis are valid — revoking means deleting the key.
type TokenStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewTokenStore(cfg *config.Config) (*TokenStore, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}
	return &TokenStore{rdb: rdb, ttl: cfg.JWT.RefreshTTL}, nil
}

// key pattern: refresh:<userID>:<tokenHash>
func refreshKey(userID, tokenHash string) string {
	return fmt.Sprintf("refresh:%s:%s", userID, tokenHash)
}

// Save persists a refresh token hash for the given user.
func (s *TokenStore) Save(ctx context.Context, userID, tokenHash string) error {
	return s.rdb.Set(ctx, refreshKey(userID, tokenHash), "1", s.ttl).Err()
}

// Exists reports whether a refresh token is still valid (not revoked / expired).
func (s *TokenStore) Exists(ctx context.Context, userID, tokenHash string) (bool, error) {
	n, err := s.rdb.Exists(ctx, refreshKey(userID, tokenHash)).Result()
	return n > 0, err
}

// Revoke deletes a specific refresh token (e.g. logout).
func (s *TokenStore) Revoke(ctx context.Context, userID, tokenHash string) error {
	return s.rdb.Del(ctx, refreshKey(userID, tokenHash)).Err()
}

// RevokeAll deletes every refresh token for a user (e.g. password change).
func (s *TokenStore) RevokeAll(ctx context.Context, userID string) error {
	iter := s.rdb.Scan(ctx, 0, fmt.Sprintf("refresh:%s:*", userID), 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keys...).Err()
}

// AddToBlocklist puts an access token JTI on a short-lived blocklist
// (for logout before the access token expires).
func (s *TokenStore) BlockAccessToken(ctx context.Context, jti string, ttl time.Duration) error {
	return s.rdb.Set(ctx, "blocklist:"+jti, "1", ttl).Err()
}

// IsBlocked reports whether an access token JTI has been blocklisted.
func (s *TokenStore) IsBlocked(ctx context.Context, jti string) (bool, error) {
	n, err := s.rdb.Exists(ctx, "blocklist:"+jti).Result()
	return n > 0, err
}
