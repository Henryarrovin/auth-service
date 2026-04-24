package handlers

import (
	"auth-service/middleware"
	"auth-service/services/auth_service"
	"context"

	authpb "auth-service/proto/authpb"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	authpb.UnimplementedAuthServiceServer
	svc    *auth_service.AuthService
	logger *zap.Logger
}

func NewAuthHandler(svc *auth_service.AuthService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, logger: logger}
}

func (h *AuthHandler) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Email == "" || req.Password == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "email, password and name are required")
	}

	user, err := h.svc.Register(ctx, auth_service.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Role:     req.Role,
	})
	if err != nil {
		log.Error("err.auth_service.register_failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "registration failed: %v", err)
	}

	return &authpb.RegisterResponse{
		UserId:  user.ID,
		Email:   user.Email,
		Message: "user registered successfully",
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.Email == "" || req.Password == "" {
		log.Warn("warn.auth_service.login_missing_fields")
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	pair, err := h.svc.Login(ctx, req.Email, req.Password)
	if err != nil {
		log.Warn("warn.auth_service.login_failed", zap.String("email", req.Email), zap.Error(err))
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return &authpb.LoginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		TokenType:    "Bearer",
	}, nil
}

func (h *AuthHandler) Refresh(ctx context.Context, req *authpb.RefreshRequest) (*authpb.RefreshResponse, error) {
	log := middleware.FromContext(ctx, h.logger)

	if req.RefreshToken == "" {
		log.Warn("warn.auth_service.refresh_missing_token")
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	pair, err := h.svc.Refresh(ctx, req.RefreshToken)
	if err != nil {
		log.Warn("warn.auth_service.refresh_failed", zap.Error(err))
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}

	return &authpb.RefreshResponse{
		AccessToken: pair.AccessToken,
		ExpiresIn:   pair.ExpiresIn,
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
