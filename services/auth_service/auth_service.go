package auth_service

import (
	"auth-service/data"
	"auth-service/middleware"
	"auth-service/models"
	"auth-service/services/jwt_service"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users      *data.UserRepository
	tokenStore *data.TokenStore
	jwt        *jwt_service.JWTService
	logger     *zap.Logger
}

func NewAuthService(users *data.UserRepository, tokenStore *data.TokenStore, jwt *jwt_service.JWTService, logger *zap.Logger) *AuthService {
	return &AuthService{users: users, tokenStore: tokenStore, jwt: jwt, logger: logger}
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*models.User, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("registering user", zap.String("email", in.Email))

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("err.auth_service.hashing_password_failed", zap.Error(err))
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	role := in.Role
	if role == "" {
		role = models.RoleUser
	}

	user := &models.User{
		Email:        in.Email,
		Name:         in.Name,
		PasswordHash: string(hash),
	}
	if err := s.users.Create(ctx, user, role); err != nil {
		log.Error("err.auth_service.creating_user_failed", zap.String("email", in.Email), zap.Error(err))
		return nil, fmt.Errorf("creating user: %w", err)
	}
	user.Roles = []models.Role{{Name: role}}
	log.Info("user registered",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
		zap.String("role", role),
	)
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("info.auth_service.login_attempt", zap.String("email", email))

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		log.Warn("warn.auth_service.user_not_found", zap.String("email", email))
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		log.Warn("warn.auth_service.invalid_password", zap.String("email", email))
		return nil, fmt.Errorf("invalid credentials")
	}

	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}

	log.Info("info.auth_service.login_successful",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
	)
	return pair, nil
}

func (s *AuthService) issuePair(ctx context.Context, user *models.User) (*TokenPair, error) {
	log := middleware.FromContext(ctx, s.logger)

	access, err := s.jwt.IssueAccessToken(user)
	if err != nil {
		log.Error("err.auth_service.issuing_access_token_failed", zap.Error(err))
		return nil, fmt.Errorf("issuing access token: %w", err)
	}

	tokenHash, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generating token hash: %w", err)
	}

	refresh, err := s.jwt.IssueRefreshToken(user.ID, tokenHash)
	if err != nil {
		log.Error("err.auth_service.issuing_refresh_token_failed", zap.Error(err))
		return nil, fmt.Errorf("issuing refresh token: %w", err)
	}

	if err := s.tokenStore.Save(ctx, user.ID, tokenHash); err != nil {
		log.Error("err.auth_service.saving_refresh_token_failed", zap.Error(err))
		return nil, fmt.Errorf("saving refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.jwt.AccessTTL().Seconds()),
	}, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
