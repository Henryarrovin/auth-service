package handlers

import (
	"context"
	"errors"

	"github.com/Henryarrovin/auth-service/middleware"
	"github.com/Henryarrovin/auth-service/services/auth_service"
	"github.com/Henryarrovin/auth-service/services/oauth_service"

	authpb "github.com/Henryarrovin/auth-service/proto/authpb"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	authpb.UnimplementedAuthServiceServer
	svc    *auth_service.AuthService
	oauth  *oauth_service.OAuthService
	logger *zap.Logger
}

func NewAuthHandler(svc *auth_service.AuthService, oauth *oauth_service.OAuthService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, oauth: oauth, logger: logger}
}

// OAuth exposes the OAuth service so main.go can wire the raw (non-gRPC-gateway)
// redirect endpoint that the provider itself calls back to
func (h *AuthHandler) OAuth() *oauth_service.OAuthService {
	return h.oauth
}

func (h *AuthHandler) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Email == "" || req.Password == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "email, password and name are required")
	}

	pending, err := h.svc.Register(ctx, auth_service.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Role:     req.Role,
	})
	if err != nil {
		log.Error("err.auth_service.register_failed", zap.Error(err))
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	return &authpb.RegisterResponse{
		UserId:               "",
		Email:                pending.Email,
		Message:              "we've sent a verification code to your email",
		VerificationRequired: true,
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Email == "" || req.Password == "" {
		log.Warn("warn.auth_service.login_missing_fields")
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	result, err := h.svc.Login(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth_service.ErrEmailNotVerified) {
			log.Warn("login blocked: email not verified", zap.String("email", req.Email))
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		log.Warn("login failed", zap.String("email", req.Email), zap.Error(err))
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// 2FA required
	if result.Requires2FA {
		return &authpb.LoginResponse{
			Requires_2Fa: true,
			TempToken:    result.TempToken,
		}, nil
	}

	// No 2FA
	return &authpb.LoginResponse{
		AccessToken:  result.Pair.AccessToken,
		RefreshToken: result.Pair.RefreshToken,
		ExpiresIn:    result.Pair.ExpiresIn,
		TokenType:    "Bearer",
	}, nil
}

func (h *AuthHandler) Refresh(ctx context.Context, req *authpb.RefreshRequest) (*authpb.RefreshResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.RefreshToken == "" {
		log.Warn("warn.auth_service.refresh_missing_token")
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	result, err := h.svc.Refresh(ctx, req.RefreshToken)
	if err != nil {
		log.Warn("warn.auth_service.refresh_failed", zap.Error(err))
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}

	return &authpb.RefreshResponse{
		AccessToken:  result.Pair.AccessToken,
		RefreshToken: result.Pair.RefreshToken,
		ExpiresIn:    result.Pair.ExpiresIn,
	}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.AccessToken == "" {
		log.Warn("warn.auth_service.logout_missing_token")
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}

	if err := h.svc.Logout(ctx, req.AccessToken); err != nil {
		log.Error("err.auth_service.logout_failed", zap.Error(err))
		return &authpb.LogoutResponse{Success: false, Message: err.Error()}, nil
	}

	return &authpb.LogoutResponse{Success: true, Message: "logged out successfully"}, nil
}

func (h *AuthHandler) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	log := middleware.FromContext(ctx, h.logger)
	log.Info("info.auth_service.validate_token_request", zap.String("service", req.ServiceName))

	result, err := h.svc.ValidateToken(ctx,
		req.Token,
		req.CanonicalMethod,
		req.CanonicalPath,
		req.CanonicalDate,
		req.ServiceName,
		req.CanonicalSig,
	)
	if err != nil {
		log.Warn("warn.auth_service.validate_token_failed", zap.Error(err))
		return &authpb.ValidateTokenResponse{
			Valid: false,
			Error: err.Error(),
		}, nil
	}

	return &authpb.ValidateTokenResponse{
		Valid:  true,
		UserId: result.UserID,
		Email:  result.Email,
		Roles:  result.Roles,
	}, nil
}

func (h *AuthHandler) AssignRole(ctx context.Context, req *authpb.AssignRoleRequest) (*authpb.AssignRoleResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.UserId == "" || req.RoleName == "" {
		log.Warn("warn.auth_service.assign_role_missing_fields")
		return nil, status.Error(codes.InvalidArgument, "user_id and role_name are required")
	}

	if err := h.svc.AssignRole(ctx, req.UserId, req.RoleName); err != nil {
		log.Error("err.auth_service.assign_role_failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "assign role: %v", err)
	}

	return &authpb.AssignRoleResponse{Success: true, Message: "role assigned"}, nil
}

func (h *AuthHandler) GetUserRoles(ctx context.Context, req *authpb.GetUserRolesRequest) (*authpb.GetUserRolesResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.UserId == "" {
		log.Warn("warn.auth_service.get_user_roles_missing_user_id")
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	names, err := h.svc.GetUserRoles(ctx, req.UserId)
	if err != nil {
		log.Error("err.auth_service.get_user_roles_failed", zap.String("user_id", req.UserId), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "get roles: %v", err)
	}

	var roles []*authpb.Role
	for _, n := range names {
		roles = append(roles, &authpb.Role{Name: n})
	}

	return &authpb.GetUserRolesResponse{Roles: roles}, nil
}

func (h *AuthHandler) ForgotPassword(ctx context.Context, req *authpb.ForgotPasswordRequest) (*authpb.ForgotPasswordResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	if err := h.svc.ForgotPassword(ctx, req.Email); err != nil {
		log.Error("forgot password failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to process request")
	}

	return &authpb.ForgotPasswordResponse{
		Message: "if your email exists you will receive a reset link",
	}, nil
}

func (h *AuthHandler) ResetPassword(ctx context.Context, req *authpb.ResetPasswordRequest) (*authpb.ResetPasswordResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Token == "" || req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "token and new_password are required")
	}

	if err := h.svc.ResetPassword(ctx, req.Token, req.NewPassword); err != nil {
		log.Warn("reset password failed", zap.Error(err))
		return &authpb.ResetPasswordResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &authpb.ResetPasswordResponse{
		Success: true,
		Message: "password reset successfully",
	}, nil
}

func (h *AuthHandler) OAuthLogin(ctx context.Context, req *authpb.OAuthLoginRequest) (*authpb.OAuthLoginResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Provider == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}

	url, err := h.oauth.GetRedirectURL(ctx, req.Provider, req.AppRedirectUri)
	if err != nil {
		log.Error("oauth redirect failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "oauth failed: %v", err)
	}

	return &authpb.OAuthLoginResponse{RedirectUrl: url}, nil
}

func (h *AuthHandler) OAuthCallback(ctx context.Context, req *authpb.OAuthCallbackRequest) (*authpb.OAuthCallbackResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Code == "" || req.State == "" {
		return nil, status.Error(codes.InvalidArgument, "code and state are required")
	}

	result, err := h.oauth.HandleCallback(ctx, req.Provider, req.Code, req.State)
	if err != nil {
		log.Error("oauth callback failed", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "oauth failed: %v", err)
	}

	return &authpb.OAuthCallbackResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		TokenType:    "Bearer",
		IsNewUser:    result.IsNewUser,
	}, nil
}

func (h *AuthHandler) VerifyOTP(ctx context.Context, req *authpb.VerifyOTPRequest) (*authpb.VerifyOTPResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.TempToken == "" || req.Otp == "" {
		log.Warn("warn.auth_service.verify_otp_missing_fields")
		return nil, status.Error(codes.InvalidArgument, "temp_token and otp are required")
	}

	pair, err := h.svc.VerifyOTP(ctx, req.TempToken, req.Otp)
	if err != nil {
		log.Warn("otp verification failed", zap.Error(err))
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	return &authpb.VerifyOTPResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		TokenType:    "Bearer",
	}, nil
}

func (h *AuthHandler) Setup2FA(ctx context.Context, req *authpb.Setup2FARequest) (*authpb.Setup2FAResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.UserId == "" {
		log.Warn("warn.auth_service.setup_2fa_missing_fields")
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	result, err := h.svc.Setup2FA(ctx, req.UserId)
	if err != nil {
		log.Error("2fa setup failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "2fa setup failed: %v", err)
	}

	return &authpb.Setup2FAResponse{
		Secret:  result.Secret,
		QrUrl:   result.QRURL,
		QrImage: result.QRImage,
	}, nil
}

func (h *AuthHandler) Enable2FA(ctx context.Context, req *authpb.Enable2FARequest) (*authpb.Enable2FAResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.UserId == "" || req.Otp == "" {
		log.Warn("warn.auth_service.enable_2fa_missing_fields")
		return nil, status.Error(codes.InvalidArgument, "user_id and otp are required")
	}

	backupCodes, err := h.svc.Enable2FA(ctx, req.UserId, req.Otp)
	if err != nil {
		log.Warn("2fa enable failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &authpb.Enable2FAResponse{
		Success:     true,
		Message:     "2FA enabled successfully",
		BackupCodes: backupCodes,
	}, nil
}

func (h *AuthHandler) Disable2FA(ctx context.Context, req *authpb.Disable2FARequest) (*authpb.Disable2FAResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.UserId == "" || req.Otp == "" {
		log.Warn("warn.auth_service.disable_2fa_missing_fields")
		return nil, status.Error(codes.InvalidArgument, "user_id and otp are required")
	}

	if err := h.svc.Disable2FA(ctx, req.UserId, req.Otp); err != nil {
		log.Warn("2fa disable failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &authpb.Disable2FAResponse{
		Success: true,
		Message: "2FA disabled successfully",
	}, nil
}

func (h *AuthHandler) SetupSyncKey(ctx context.Context, req *authpb.SetupSyncKeyRequest) (*authpb.SetupSyncKeyResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.AccessToken == "" || req.Salt == "" || req.WrappedDek == "" || req.WrappedDekNonce == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token, salt, wrapped_dek and wrapped_dek_nonce are required")
	}

	if err := h.svc.SetupSyncKey(ctx, req.AccessToken, req.Salt, req.KdfParams, req.WrappedDek, req.WrappedDekNonce); err != nil {
		log.Warn("warn.auth_service.setup_sync_key_failed", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	return &authpb.SetupSyncKeyResponse{Success: true, Message: "sync key configured"}, nil
}

func (h *AuthHandler) GetSyncKey(ctx context.Context, req *authpb.GetSyncKeyRequest) (*authpb.GetSyncKeyResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.AccessToken == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}

	material, err := h.svc.GetSyncKey(ctx, req.AccessToken)
	if err != nil {
		log.Warn("warn.auth_service.get_sync_key_failed", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	return &authpb.GetSyncKeyResponse{
		Configured:      material.Configured,
		Salt:            material.Salt,
		KdfParams:       material.KDFParams,
		WrappedDek:      material.WrappedDEK,
		WrappedDekNonce: material.WrappedDEKNonce,
	}, nil
}

func (h *AuthHandler) VerifyEmail(ctx context.Context, req *authpb.VerifyEmailRequest) (*authpb.VerifyEmailResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Email == "" || req.Otp == "" {
		return nil, status.Error(codes.InvalidArgument, "email and otp are required")
	}

	pair, err := h.svc.VerifyEmail(ctx, req.Email, req.Otp)
	if err != nil {
		log.Warn("email verification failed", zap.String("email", req.Email), zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &authpb.VerifyEmailResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		TokenType:    "Bearer",
		Message:      "email verified successfully",
	}, nil
}

func (h *AuthHandler) ResendVerification(ctx context.Context, req *authpb.ResendVerificationRequest) (*authpb.ResendVerificationResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	if err := h.svc.ResendVerification(ctx, req.Email); err != nil {
		log.Error("resend verification failed", zap.String("email", req.Email), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to resend verification email")
	}

	return &authpb.ResendVerificationResponse{
		Message: "if this email needs verification, a new code has been sent",
	}, nil
}
