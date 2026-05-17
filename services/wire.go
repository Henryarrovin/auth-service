package services

import (
	"github.com/Henryarrovin/auth-service/services/auth_service"
	"github.com/Henryarrovin/auth-service/services/email_service"
	"github.com/Henryarrovin/auth-service/services/jwt_service"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	jwt_service.ProviderSet,
	auth_service.ProviderSet,
	email_service.ProviderSet,
)
