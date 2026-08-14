package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Henryarrovin/auth-service/config"

	"github.com/redis/go-redis/v9"
)

type PendingRegistration struct {
	Name         string `json:"name"`
	PasswordHash string `json:"passwordHash"`
	Role         string `json:"role"`
}

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

// SaveResetToken stores a password reset token with 15min TTL
func (s *TokenStore) SaveResetToken(ctx context.Context, email, token string) error {
	key := fmt.Sprintf("reset:%s", token)
	return s.rdb.Set(ctx, key, email, 15*time.Minute).Err()
}

// GetResetToken returns the email associated with a reset token
func (s *TokenStore) GetResetToken(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("reset:%s", token)
	email, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("reset token expired or invalid")
	}
	return email, err
}

// DeleteResetToken removes the reset token after use
func (s *TokenStore) DeleteResetToken(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, fmt.Sprintf("reset:%s", token)).Err()
}

type oauthStateValue struct {
	Provider       string `json:"provider"`
	AppRedirectURI string `json:"app_redirect_uri,omitempty"`
}

// SaveOAuthState stores OAuth state for CSRF protection
func (s *TokenStore) SaveOAuthState(ctx context.Context, state, provider, appRedirectURI string) error {
	val, err := json.Marshal(oauthStateValue{Provider: provider, AppRedirectURI: appRedirectURI})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, "oauth:state:"+state, val, 10*time.Minute).Err()
}

// GetOAuthState retrieves the provider for a given state
func (s *TokenStore) GetOAuthState(ctx context.Context, state string) (provider, appRedirectURI string, err error) {
	raw, err := s.rdb.Get(ctx, "oauth:state:"+state).Result()
	if err == redis.Nil {
		return "", "", fmt.Errorf("oauth state expired or invalid")
	}
	if err != nil {
		return "", "", err
	}
	var val oauthStateValue
	if jsonErr := json.Unmarshal([]byte(raw), &val); jsonErr != nil {
		return raw, "", nil
	}
	return val.Provider, val.AppRedirectURI, nil
}

// DeleteOAuthState removes the state after use
func (s *TokenStore) DeleteOAuthState(ctx context.Context, state string) error {
	return s.rdb.Del(ctx, "oauth:state:"+state).Err()
}

// SaveTempToken stores a temporary token after first factor auth
// Used to track that user passed password check, waiting for OTP
func (s *TokenStore) SaveTempToken(ctx context.Context, tempToken, userID string) error {
	return s.rdb.Set(ctx, "2fa:temp:"+tempToken, userID, 5*time.Minute).Err()
}

// GetTempToken returns userID for a temp token
func (s *TokenStore) GetTempToken(ctx context.Context, tempToken string) (string, error) {
	userID, err := s.rdb.Get(ctx, "2fa:temp:"+tempToken).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("temp token expired or invalid")
	}
	return userID, err
}

// DeleteTempToken removes temp token after OTP verified
func (s *TokenStore) DeleteTempToken(ctx context.Context, tempToken string) error {
	return s.rdb.Del(ctx, "2fa:temp:"+tempToken).Err()
}

// SaveEmailOTP stores an email verification code with a 10min TTL
func (s *TokenStore) SaveEmailOTP(ctx context.Context, email, otp string) error {
	key := fmt.Sprintf("emailverify:%s", email)
	return s.rdb.Set(ctx, key, otp, 10*time.Minute).Err()
}

// GetEmailOTP returns the code stored for an email
func (s *TokenStore) GetEmailOTP(ctx context.Context, email string) (string, error) {
	key := fmt.Sprintf("emailverify:%s", email)
	otp, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("verification code expired or invalid")
	}
	return otp, err
}

// DeleteEmailOTP removes the code after it's used
func (s *TokenStore) DeleteEmailOTP(ctx context.Context, email string) error {
	return s.rdb.Del(ctx, fmt.Sprintf("emailverify:%s", email)).Err()
}

func (s *TokenStore) SavePendingRegistration(ctx context.Context, email string, reg PendingRegistration) error {
	data, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("marshal pending registration: %w", err)
	}
	key := fmt.Sprintf("pendingreg:%s", email)
	return s.rdb.Set(ctx, key, data, 15*time.Minute).Err()
}

func (s *TokenStore) GetPendingRegistration(ctx context.Context, email string) (*PendingRegistration, error) {
	key := fmt.Sprintf("pendingreg:%s", email)
	data, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("signup expired or not found — please sign up again")
	}
	if err != nil {
		return nil, err
	}
	var reg PendingRegistration
	if err := json.Unmarshal([]byte(data), &reg); err != nil {
		return nil, fmt.Errorf("unmarshal pending registration: %w", err)
	}
	return &reg, nil
}

func (s *TokenStore) DeletePendingRegistration(ctx context.Context, email string) error {
	return s.rdb.Del(ctx, fmt.Sprintf("pendingreg:%s", email)).Err()
}
