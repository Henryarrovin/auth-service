package auth_service

import (
	"auth-service/data"
	"auth-service/models"
	"auth-service/services/jwt_service"
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users      *data.UserRepository
	tokenStore *data.TokenStore
	jwt        *jwt_service.JWTService
}

func NewAuthService(users *data.UserRepository, tokenStore *data.TokenStore, jwt *jwt_service.JWTService) *AuthService {
	return &AuthService{users: users, tokenStore: tokenStore, jwt: jwt}
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
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
		return nil, fmt.Errorf("creating user: %w", err)
	}
	user.Roles = []models.Role{{Name: role}}
	return user, nil
}
