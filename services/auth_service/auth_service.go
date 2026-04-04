package auth_service

import (
	"auth-service/data"
	"auth-service/middleware"
	"auth-service/models"
	"auth-service/services/jwt_service"
	"context"
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
	return user, nil
}
