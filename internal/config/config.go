package config

import (
	"fmt"
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds the application's configuration values.
// Tags like `envconfig:"APP_PORT"` specify the environment variable name.
// `default:""` provides a default value if the env var is not set.
// `required:"true"` makes an environment variable mandatory.
type Config struct {
	AppEnv     string `envconfig:"APP_ENV" default:"development"` // e.g., development, staging, production
	LogLevel   string `envconfig:"LOG_LEVEL" default:"info"`    // e.g., debug, info, warn, error
	HttpServer ServerConfig
	GrpcServer GrpcServerConfig
	Postgres   PostgresConfig
	// Add other configurations like JWT secrets, external service URLs, etc.
	// JWTSecret string `envconfig:"JWT_SECRET" required:"true"`
}

// ServerConfig holds HTTP server-specific configurations.
type ServerConfig struct {
	Port         string        `envconfig:"HTTP_SERVER_PORT" default:"8080"`
	TimeoutRead  time.Duration `envconfig:"HTTP_SERVER_TIMEOUT_READ" default:"15s"`
	TimeoutWrite time.Duration `envconfig:"HTTP_SERVER_TIMEOUT_WRITE" default:"15s"`
	TimeoutIdle  time.Duration `envconfig:"HTTP_SERVER_TIMEOUT_IDLE" default:"60s"`
	// BasePath string 		`envconfig:"HTTP_SERVER_BASE_PATH" default:"/api/v1"` // Matches your OpenAPI
}

// GrpcServerConfig holds gRPC server-specific configurations.
type GrpcServerConfig struct {
	Port string `envconfig:"GRPC_SERVER_PORT" default:"9090"`
	// Add other gRPC specific settings if needed, e.g., max message size
}

// PostgresConfig holds PostgreSQL database connection details.
type PostgresConfig struct {
    Host     string `envconfig:"POSTGRES_HOST" required:"true"`
    Port     string `envconfig:"POSTGRES_PORT" default:"5432"`
    User     string `envconfig:"POSTGRES_USER" required:"true"`
    Password string `envconfig:"POSTGRES_PASSWORD" required:"true"`
    DBName   string `envconfig:"POSTGRES_DBNAME" required:"true"`
}

// DSN constructs the Data Source Name string for connecting to PostgreSQL.
func (pc *PostgresConfig) DSN() string {
	// Example: "host=localhost port=5432 user=user password=password dbname=mydb sslmode=disable"
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		pc.Host, pc.Port, pc.User, pc.Password, pc.DBName)
}

var cfg Config

// Load initializes the configuration from environment variables.
// It should be called once during application startup.
func Load() (*Config, error) {
	log.Println("Loading service configuration...")
	err := envconfig.Process("", &cfg) // The first argument is a prefix for env vars, empty means no prefix
	if err != nil {
		return nil, fmt.Errorf("failed to process configuration: %w", err)
	}

	// You can add custom validation here if needed, e.g.:
	// if cfg.AppEnv != "development" && cfg.AppEnv != "staging" && cfg.AppEnv != "production" {
	// 	return nil, fmt.Errorf("invalid APP_ENV: %s", cfg.AppEnv)
	// }

	log.Printf("Configuration loaded successfully for APP_ENV: %s", cfg.AppEnv)
	// For security, avoid logging sensitive parts of the config like passwords or full DSNs in production.
	// log.Printf("Postgres DSN (example, careful with logging this): %s", cfg.Postgres.DSN())
	return &cfg, nil
}

// Get returns the loaded configuration.
// Panics if Load() has not been called successfully.
func Get() *Config {
	if cfg.Postgres.Host == "" { // Simple check to see if cfg is populated
		log.Fatal("Configuration has not been loaded. Call config.Load() first.")
	}
	return &cfg
}