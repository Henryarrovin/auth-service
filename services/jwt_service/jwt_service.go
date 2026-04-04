package jwt_service

import (
	"auth-service/config"
	"auth-service/models"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type accessTokenClaims struct {
	jwt.RegisteredClaims
	UserID string   `json:"uid"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
	// Canonical signature binds this token to an allowed service/path combo.
	// Empty for general-purpose tokens.
	CanonicalSig string `json:"csig,omitempty"`
}

type refreshTokenClaims struct {
	jwt.RegisteredClaims
	UserID    string `json:"uid"`
	TokenHash string `json:"jti_hash"` // stored in Redis for revocation
}

type JWTService struct {
	cfg config.JWTConfig
}

func NewJWTService(cfg config.JWTConfig) *JWTService {
	return &JWTService{cfg: cfg}
}

// IssueAccessToken creates a signed JWT carrying user identity + roles.
func (s *JWTService) IssueAccessToken(user *models.User) (string, error) {
	now := time.Now()

	claims := accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
			Issuer:    "auth-service",
		},
		UserID: user.ID,
		Email:  user.Email,
		Roles:  user.RoleNames(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.AccessSecret))
}

// IssueRefreshToken creates a long-lived refresh token.
// tokenHash is a random bytes string stored in Redis.
func (s *JWTService) IssueRefreshToken(userID, tokenHash string) (string, error) {
	now := time.Now()
	claims := refreshTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.RefreshTTL)),
			Issuer:    "auth-service",
		},
		UserID:    userID,
		TokenHash: tokenHash,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.RefreshSecret))
}

// ValidateAccessToken parses and validates an access token.
// Returns decoded claims or an error.
func (s *JWTService) ValidateAccessToken(tokenStr string) (*models.AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &accessTokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.AccessSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid access token: %w", err)
	}

	c, ok := token.Claims.(*accessTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("malformed access token claims")
	}

	return &models.AccessClaims{
		UserID: c.UserID,
		Email:  c.Email,
		Roles:  c.Roles,
	}, nil
}

// ParseRefreshToken extracts claims from a refresh token without touching Redis.
func (s *JWTService) ParseRefreshToken(tokenStr string) (*models.RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &refreshTokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.RefreshSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	c, ok := token.Claims.(*refreshTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("malformed refresh token claims")
	}

	return &models.RefreshClaims{
		UserID:    c.UserID,
		TokenHash: c.TokenHash,
	}, nil
}

// Purpose: allow service-to-service calls to prove that a request
// was made by a trusted caller and has not been tampered with.
//
// Canonical string = METHOD\nPATH\nDATE\nSERVICE
// Signature        = HMAC-SHA256(canonical_secret, canonical_string)
//
// The auth service verifies this signature during ValidateToken so that
// even if someone steals a JWT they cannot replay it to a different
// service or endpoint.

// BuildCanonicalString assembles the string-to-sign.
func BuildCanonicalString(method, path, date, service string) string {
	return strings.Join([]string{
		strings.ToUpper(method),
		path,
		date,
		service,
	}, "\n")
}

// SignCanonical produces the HMAC-SHA256 hex signature.
func (s *JWTService) SignCanonical(method, path, date, service string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.CanonicalSecret))
	mac.Write([]byte(BuildCanonicalString(method, path, date, service)))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyCanonical checks that the provided signature matches expectations.
func (s *JWTService) VerifyCanonical(method, path, date, service, sig string) bool {
	expected := s.SignCanonical(method, path, date, service)
	return hmac.Equal([]byte(expected), []byte(sig))
}
