package wire

import (
	"auth-service/config"
	"auth-service/data"
	"auth-service/handlers"
	"auth-service/services"

	"github.com/google/wire"
	"go.uber.org/zap"
)

func InitializeContainer(cfgFile string, logger *zap.Logger) (*handlers.AuthHandler, error) {
	wire.Build(
		config.ProviderSet,
		data.ProviderSet,
		services.ProviderSet,
		handlers.ProviderSet,
	)
	return nil, nil
}
