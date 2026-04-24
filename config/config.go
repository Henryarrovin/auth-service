package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
	GroupID string   `mapstructure:"group_id"`
	LogDir  string   `mapstructure:"log_dir"`
	Enabled bool     `mapstructure:"enabled"`
}

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Kafka    KafkaConfig
}

type ServerConfig struct {
	GRPCPort int    `mapstructure:"grpc_port"`
	Env      string `mapstructure:"env"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	// AccessSecret signs the short-lived access token
	AccessSecret string `mapstructure:"access_secret"`
	// RefreshSecret signs the long-lived refresh token
	RefreshSecret string `mapstructure:"refresh_secret"`
	// CanonicalSecret signs inter-service canonical requests
	CanonicalSecret string `mapstructure:"canonical_secret"`

	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

// Load reads config from a file and / or environment variables.
// Environment variables override file values.
// Prefix: AUTH_  (e.g. AUTH_SERVER_GRPC_PORT=50051)
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// ── defaults ──────────────────────────────────
	v.SetDefault("server.grpc_port", 50051)
	v.SetDefault("server.env", "development")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("jwt.access_ttl", "15m")
	v.SetDefault("jwt.refresh_ttl", "168h")

	// ── file ──────────────────────────────────────
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// ── env ───────────────────────────────────────
	v.SetEnvPrefix("AUTH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// ── kafka ──────────────────────────────────
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.topic", "auth-service-logs")
	v.SetDefault("kafka.group_id", "auth-log-consumer")
	v.SetDefault("kafka.log_dir", "./logs")
	v.SetDefault("kafka.enabled", false)

	// manually read kafka fields from env
	// Viper doesn't handle []string and bool from env vars reliably
	if enabled := os.Getenv("AUTH_KAFKA_ENABLED"); enabled == "true" {
		cfg.Kafka.Enabled = true
	}

	if brokers := os.Getenv("AUTH_KAFKA_BROKERS"); brokers != "" {
		cfg.Kafka.Brokers = strings.Split(brokers, ",")
	}

	if topic := os.Getenv("AUTH_KAFKA_TOPIC"); topic != "" {
		cfg.Kafka.Topic = topic
	}

	if groupID := os.Getenv("AUTH_KAFKA_GROUP_ID"); groupID != "" {
		cfg.Kafka.GroupID = groupID
	}

	if logDir := os.Getenv("AUTH_KAFKA_LOG_DIR"); logDir != "" {
		cfg.Kafka.LogDir = logDir
	}

	return &cfg, nil
}
