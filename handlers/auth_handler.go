package handlers

import (
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
		h.logger.Error("register failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "registration failed: %v", err)
	}

	return &authpb.RegisterResponse{
		UserId:  user.ID,
		Email:   user.Email,
		Message: "user registered successfully",
	}, nil
}
