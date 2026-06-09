package config

import (
	"fmt"
	"os"
	"strconv"
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

type EmailConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	BaseURL  string `mapstructure:"base_url"`
}

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Kafka    KafkaConfig
	Email    EmailConfig
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
	v.SetDefault("kafka.enabled", false)
	v.SetDefault("kafka.topic", "auth-service-logs")
	v.SetDefault("kafka.group_id", "auth-log-consumer")
	v.SetDefault("kafka.log_dir", "./logs")
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("email.host", "smtp.gmail.com")
	v.SetDefault("email.port", 587)
	v.SetDefault("email.base_url", "http://localhost:8080")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

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

	// ── Manually read ALL env vars ────────────────
	// Viper AutomaticEnv is unreliable with nested structs
	if v := os.Getenv("AUTH_DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("AUTH_DB_PORT"); v != "" {
		p, _ := strconv.Atoi(v)
		cfg.Database.Port = p
	}
	if v := os.Getenv("AUTH_DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("AUTH_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("AUTH_DB_NAME"); v != "" {
		cfg.Database.DBName = v
	}
	if v := os.Getenv("AUTH_DB_SSLMODE"); v != "" {
		cfg.Database.SSLMode = v
	}
	if v := os.Getenv("AUTH_REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("AUTH_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("AUTH_REDIS_DB"); v != "" {
		d, _ := strconv.Atoi(v)
		cfg.Redis.DB = d
	}
	if v := os.Getenv("AUTH_SERVER_GRPC_PORT"); v != "" {
		p, _ := strconv.Atoi(v)
		cfg.Server.GRPCPort = p
	}
	if v := os.Getenv("AUTH_SERVER_ENV"); v != "" {
		cfg.Server.Env = v
	}
	if v := os.Getenv("AUTH_JWT_ACCESS_SECRET"); v != "" {
		cfg.JWT.AccessSecret = v
	}
	if v := os.Getenv("AUTH_JWT_REFRESH_SECRET"); v != "" {
		cfg.JWT.RefreshSecret = v
	}
	if v := os.Getenv("AUTH_JWT_CANONICAL_SECRET"); v != "" {
		cfg.JWT.CanonicalSecret = v
	}
	if v := os.Getenv("AUTH_JWT_ACCESS_TTL"); v != "" {
		d, _ := time.ParseDuration(v)
		cfg.JWT.AccessTTL = d
	}
	if v := os.Getenv("AUTH_JWT_REFRESH_TTL"); v != "" {
		d, _ := time.ParseDuration(v)
		cfg.JWT.RefreshTTL = d
	}
	if v := os.Getenv("AUTH_KAFKA_ENABLED"); v != "" {
		cfg.Kafka.Enabled = v == "true"
	}
	if v := os.Getenv("AUTH_KAFKA_BROKERS"); v != "" {
		cfg.Kafka.Brokers = strings.Split(v, ",")
	}
	if v := os.Getenv("AUTH_KAFKA_TOPIC"); v != "" {
		cfg.Kafka.Topic = v
	}
	if v := os.Getenv("AUTH_KAFKA_GROUP_ID"); v != "" {
		cfg.Kafka.GroupID = v
	}
	if v := os.Getenv("AUTH_KAFKA_LOG_DIR"); v != "" {
		cfg.Kafka.LogDir = v
	}
	if v := os.Getenv("AUTH_EMAIL_HOST"); v != "" {
		cfg.Email.Host = v
	}
	if v := os.Getenv("AUTH_EMAIL_PORT"); v != "" {
		p, _ := strconv.Atoi(v)
		cfg.Email.Port = p
	}
	if v := os.Getenv("AUTH_EMAIL_USERNAME"); v != "" {
		cfg.Email.Username = v
	}
	if v := os.Getenv("AUTH_EMAIL_PASSWORD"); v != "" {
		cfg.Email.Password = v
	}
	if v := os.Getenv("AUTH_EMAIL_FROM"); v != "" {
		cfg.Email.From = v
	}
	if v := os.Getenv("AUTH_EMAIL_BASE_URL"); v != "" {
		cfg.Email.BaseURL = v
	}

	return &cfg, nil
}
