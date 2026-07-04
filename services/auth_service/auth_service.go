package auth_service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/Henryarrovin/auth-service/data"
	"github.com/Henryarrovin/auth-service/middleware"
	"github.com/Henryarrovin/auth-service/models"
	"github.com/Henryarrovin/auth-service/services/email_service"
	"github.com/Henryarrovin/auth-service/services/jwt_service"
	"github.com/Henryarrovin/auth-service/services/totp_service"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users      *data.UserRepository
	tokenStore *data.TokenStore
	jwt        *jwt_service.JWTService
	totp       *totp_service.TOTPService
	email      *email_service.EmailService
	logger     *zap.Logger
}

func NewAuthService(
	users *data.UserRepository,
	tokenStore *data.TokenStore,
	jwt *jwt_service.JWTService,
	totp *totp_service.TOTPService,
	email *email_service.EmailService,
	logger *zap.Logger,
) *AuthService {
	return &AuthService{users: users, tokenStore: tokenStore, jwt: jwt, totp: totp, email: email, logger: logger}
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

func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("info.auth_service.login_attempt", zap.String("email", email))

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		log.Warn("warn.auth_service.user_not_found", zap.String("email", email))
		return nil, fmt.Errorf("invalid credentials")
	}

	if user.PasswordHash == "" {
		providers := make([]string, len(user.Providers))
		for i, p := range user.Providers {
			providers[i] = p.Provider
		}
		return nil, fmt.Errorf("this account uses %s login", strings.Join(providers, "/"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		log.Warn("warn.auth_service.invalid_password", zap.String("email", email))
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if 2FA is enabled
	if user.TwoFAEnabled {
		log.Info("info.auth_service.2fa_required", zap.String("user_id", user.ID))

		tempToken, err := randomHex(32)
		if err != nil {
			return nil, fmt.Errorf("generating temp token: %w", err)
		}

		if err := s.tokenStore.SaveTempToken(ctx, tempToken, user.ID); err != nil {
			return nil, fmt.Errorf("saving temp token: %w", err)
		}

		return &LoginResult{
			Requires2FA: true,
			TempToken:   tempToken,
		}, nil
	}

	// No 2FA — issue pair directly
	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}

	log.Info("info.auth_service.login_successful", zap.String("user_id", user.ID))
	return &LoginResult{Pair: pair}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshTokenStr string) (*RefreshResult, error) {
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

	// require 2FA again before issuing new tokens
	/*
		if user.TwoFAEnabled {
			tempToken, err := randomHex(32)
			if err != nil {
				return nil, fmt.Errorf("generating temp token: %w", err)
			}

			if err := s.tokenStore.SaveTempToken(ctx, tempToken, user.ID); err != nil {
				return nil, fmt.Errorf("saving temp token: %w", err)
			}

			return &RefreshResult{
				Requires2FA: true,
				TempToken:   tempToken,
			}, nil
		}
	*/

	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}

	log.Info("info.auth_service.refresh_successful", zap.String("user_id", user.ID))

	return &RefreshResult{
		Pair: pair,
	}, nil
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

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("forgot password request", zap.String("email", email))

	_, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		log.Warn("forgot password for unknown email", zap.String("email", email))
		return nil
	}

	token, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("generating reset token: %w", err)
	}

	if err := s.tokenStore.SaveResetToken(ctx, email, token); err != nil {
		return fmt.Errorf("saving reset token: %w", err)
	}

	if err := s.email.SendPasswordReset(email, token); err != nil {
		return fmt.Errorf("sending reset email: %w", err)
	}

	log.Info("password reset email sent", zap.String("email", email))
	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("reset password attempt")

	email, err := s.tokenStore.GetResetToken(ctx, token)
	if err != nil {
		log.Warn("invalid or expired reset token", zap.Error(err))
		return fmt.Errorf("invalid or expired reset token")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	if err := s.users.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		log.Error("updating password failed", zap.Error(err))
		return fmt.Errorf("updating password: %w", err)
	}

	_ = s.tokenStore.DeleteResetToken(ctx, token)

	_ = s.tokenStore.RevokeAll(ctx, user.ID)

	log.Info("password reset successful",
		zap.String("user_id", user.ID),
		zap.String("email", email),
	)
	return nil
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

func (s *AuthService) VerifyOTP(ctx context.Context, tempToken, otpCode string) (*TokenPair, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("verifying otp")

	// Get userID from temp token
	userID, err := s.tokenStore.GetTempToken(ctx, tempToken)
	if err != nil {
		log.Warn("invalid temp token", zap.Error(err))
		return nil, fmt.Errorf("invalid or expired session")
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Validate OTP
	valid := s.totp.ValidateOTP(user.TwoFASecret, otpCode)

	// If OTP invalid, try backup code
	if !valid && len(user.TwoFABackupCodes) > 0 {
		var newCodesJSON string
		valid, newCodesJSON, err = s.totp.ValidateBackupCode(user.TwoFABackupCodes, otpCode)
		if err != nil {
			log.Error("backup code validation error", zap.Error(err))
		}
		if valid {
			// Update backup codes (remove used one)
			_ = s.users.UpdateBackupCodes(ctx, userID, newCodesJSON)
			log.Info("backup code used", zap.String("user_id", userID))
		}
	}

	if !valid {
		log.Warn("invalid otp", zap.String("user_id", userID))
		return nil, fmt.Errorf("invalid OTP")
	}

	// Delete temp token
	_ = s.tokenStore.DeleteTempToken(ctx, tempToken)

	// Issue full token pair
	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}

	log.Info("otp verified, login successful", zap.String("user_id", userID))
	return pair, nil
}

func (s *AuthService) Setup2FA(ctx context.Context, userID string) (*Setup2FAResult, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("setting up 2fa", zap.String("user_id", userID))

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if user.TwoFAEnabled {
		return nil, fmt.Errorf("2FA is already enabled")
	}

	secret, qrURL, qrImage, err := s.totp.GenerateSecret(user.Email)
	if err != nil {
		return nil, fmt.Errorf("generating 2fa secret: %w", err)
	}

	// Store secret temporarily (not enabled yet)
	if err := s.users.Update2FASecret(ctx, userID, secret); err != nil {
		return nil, fmt.Errorf("storing 2fa secret: %w", err)
	}

	log.Info("2fa secret generated", zap.String("user_id", userID))
	return &Setup2FAResult{
		Secret:  secret,
		QRURL:   qrURL,
		QRImage: qrImage,
	}, nil
}

func (s *AuthService) Enable2FA(ctx context.Context, userID, otpCode string) ([]string, error) {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("enabling 2fa", zap.String("user_id", userID))

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if user.TwoFASecret == "" {
		return nil, fmt.Errorf("2FA not set up — call /2fa/setup first")
	}

	// Verify OTP before enabling
	if !s.totp.ValidateOTP(user.TwoFASecret, otpCode) {
		log.Warn("invalid otp during 2fa enable", zap.String("user_id", userID))
		return nil, fmt.Errorf("invalid OTP")
	}

	// Generate backup codes
	backupCodes, hashedJSON, err := s.totp.GenerateBackupCodes()
	if err != nil {
		return nil, fmt.Errorf("generating backup codes: %w", err)
	}

	if err := s.users.Enable2FA(ctx, userID, hashedJSON); err != nil {
		return nil, fmt.Errorf("enabling 2fa: %w", err)
	}

	log.Info("2fa enabled", zap.String("user_id", userID))
	return backupCodes, nil
}

func (s *AuthService) Disable2FA(ctx context.Context, userID, otpCode string) error {
	log := middleware.FromContext(ctx, s.logger)
	log.Info("disabling 2fa", zap.String("user_id", userID))

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if !user.TwoFAEnabled {
		return fmt.Errorf("2FA is not enabled")
	}

	// Verify OTP before disabling
	if !s.totp.ValidateOTP(user.TwoFASecret, otpCode) {
		log.Warn("invalid otp during 2fa disable", zap.String("user_id", userID))
		return fmt.Errorf("invalid OTP")
	}

	if err := s.users.Disable2FA(ctx, userID); err != nil {
		return fmt.Errorf("disabling 2fa: %w", err)
	}

	log.Info("2fa disabled", zap.String("user_id", userID))
	return nil
}

// SetupSyncKey stores a client-wrapped data key for the caller (identified by their access token)
// The server never sees the passphrase, the derived key-encryption key, or the plaintext data key
func (s *AuthService) SetupSyncKey(ctx context.Context, accessToken, salt, kdfParams, wrappedDEK, wrappedNonce string) error {
	claims, err := s.jwt.ValidateAccessToken(accessToken)
	if err != nil {
		return fmt.Errorf("invalid access token: %w", err)
	}
	if salt == "" || wrappedDEK == "" || wrappedNonce == "" {
		return fmt.Errorf("salt, wrapped_dek and wrapped_dek_nonce are required")
	}
	return s.users.SetSyncKeyMaterial(ctx, claims.UserID, salt, kdfParams, wrappedDEK, wrappedNonce)
}

func (s *AuthService) GetSyncKey(ctx context.Context, accessToken string) (*SyncKeyMaterial, error) {
	claims, err := s.jwt.ValidateAccessToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid access token: %w", err)
	}
	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if user.WrappedDEK == "" {
		return &SyncKeyMaterial{Configured: false}, nil
	}
	return &SyncKeyMaterial{
		Configured:      true,
		Salt:            user.SyncKeySalt,
		KDFParams:       user.SyncKDFParams,
		WrappedDEK:      user.WrappedDEK,
		WrappedDEKNonce: user.WrappedDEKNonce,
	}, nil
}
