//go:build wireinject

package wire

import (
	"auth-service/config"
	"auth-service/data"
	"auth-service/handlers"
	"auth-service/services"

	"github.com/google/wire"
	"go.uber.org/zap"
)

func buildLogger(env string) (*zap.Logger, error) {
	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	cfg.EncoderConfig.CallerKey = "caller"
	cfg.Development = true

	return cfg.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(0),
	)
}

func InitializeContainer(cfgFile string, logger *zap.Logger) (*handlers.AuthHandler, func(), error) {
	wire.Build(
		config.ProviderSet,
		data.ProviderSet,
		services.ProviderSet,
		handlers.ProviderSet,
	)
	return nil, nil, nil
}
