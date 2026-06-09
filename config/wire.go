package config

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	Load,
	ProvideJWTConfig,
	ProvideServerConfig,
	ProvideDatabaseConfig,
	ProvideRedisConfig,
	ProvideKafkaConfig,
	ProvideEmailConfig,
)

func ProvideJWTConfig(cfg *Config) JWTConfig {
	return cfg.JWT
}

func ProvideServerConfig(cfg *Config) ServerConfig {
	return cfg.Server
}

func ProvideDatabaseConfig(cfg *Config) DatabaseConfig {
	return cfg.Database
}

func ProvideRedisConfig(cfg *Config) RedisConfig {
	return cfg.Redis
}

func ProvideKafkaConfig(cfg *Config) KafkaConfig {
	return cfg.Kafka
}

func ProvideEmailConfig(cfg *Config) EmailConfig {
	return cfg.Email
}
