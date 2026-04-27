package auth_service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/Henryarrovin/auth-service/data"
	"github.com/Henryarrovin/auth-service/middleware"
	"github.com/Henryarrovin/auth-service/models"
	"github.com/Henryarrovin/auth-service/services/jwt_service"

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

func (s *AuthService) Refresh(ctx context.Context, refreshTokenStr string) (*TokenPair, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("info.auth_service.refresh_attempt")

	claims, err := s.jwt.ParseRefreshToken(refreshTokenStr)
	if err != nil {
		log.Warn("warn.auth_service.refresh_invalid_token", zap.Error(err))
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	ok, err := s.tokenStore.Exists(ctx, claims.UserID, claims.TokenHash)
	if err != nil {
		log.Error("err.auth_service.refresh_token_store_check_failed", zap.Error(err))
		return nil, fmt.Errorf("checking token store: %w", err)
	}
	if !ok {
		log.Warn("warn.auth_service.refresh_token_revoked", zap.String("user_id", claims.UserID))
		return nil, fmt.Errorf("refresh token has been revoked")
	}

	_ = s.tokenStore.Revoke(ctx, claims.UserID, claims.TokenHash)

	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		log.Error("err.auth_service.refresh_user_not_found", zap.String("user_id", claims.UserID))
		return nil, fmt.Errorf("user not found: %w", err)
	}

	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}

	log.Info("info.auth_service.refresh_successful", zap.String("user_id", user.ID))
	return pair, nil
}

func (s *AuthService) Logout(ctx context.Context, accessTokenStr string) error {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("info.auth_service.logout_attempt")

	claims, err := s.jwt.ValidateAccessToken(accessTokenStr)
	if err != nil {
		log.Warn("warn.auth_service.invalid_token_on_logout", zap.Error(err))
		return fmt.Errorf("invalid token: %w", err)
	}

	if err := s.tokenStore.RevokeAll(ctx, claims.UserID); err != nil {
		log.Error("err.auth_service.revoke_tokens_failed", zap.String("user_id", claims.UserID), zap.Error(err))
		return err
	}

	log.Info("info.auth_service.logout_successful", zap.String("user_id", claims.UserID))
	return nil
}

func (s *AuthService) ValidateToken(ctx context.Context, tokenStr, method, path, date, service, canonicalSig string) (*ValidateResult, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("info.auth_service.validating_token",
		zap.String("service", service),
		zap.String("method", method),
		zap.String("path", path),
	)

	claims, err := s.jwt.ValidateAccessToken(tokenStr)
	if err != nil {
		log.Warn("warn.auth_service.token_validation_failed", zap.Error(err))
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if method != "" && path != "" && date != "" && service != "" {
		if !s.jwt.VerifyCanonical(method, path, date, service, canonicalSig) {
			log.Warn("warn.auth_service.canonical_signature_invalid",
				zap.String("service", service),
				zap.String("method", method),
				zap.String("path", path),
			)
			return nil, fmt.Errorf("canonical request signature invalid")
		}
	}

	log.Info("info.auth_service.token_validated",
		zap.String("user_id", claims.UserID),
		zap.Strings("roles", claims.Roles),
	)

	return &ValidateResult{
		UserID: claims.UserID,
		Email:  claims.Email,
		Roles:  claims.Roles,
	}, nil
}

func (s *AuthService) AssignRole(ctx context.Context, userID, roleName string) error {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("info.auth_service.assigning_role", zap.String("user_id", userID), zap.String("role", roleName))

	if err := s.users.AssignRole(ctx, userID, roleName); err != nil {
		log.Error("err.auth_service.assign_role_failed", zap.Error(err))
		return err
	}

	log.Info("info.auth_service.role_assigned", zap.String("user_id", userID), zap.String("role", roleName))
	return nil
}

func (s *AuthService) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("info.auth_service.getting_user_roles", zap.String("user_id", userID))

	roles, err := s.users.GetRoles(ctx, userID)
	if err != nil {
		log.Error("err.auth_service.get_user_roles_failed", zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	log.Info("info.auth_service.got_user_roles", zap.String("user_id", userID), zap.Strings("roles", roles))
	return roles, nil
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
