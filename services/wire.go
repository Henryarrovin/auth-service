package services

import (
	"auth-service/services/auth_service"
	"auth-service/services/jwt_service"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	jwt_service.ProviderSet,
	auth_service.ProviderSet,
)
