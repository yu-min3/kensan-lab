package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Telemetry TelemetryConfig
}

type ServerConfig struct {
	Host string
	Port int
	Env  string // "development", "production"
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type JWTConfig struct {
	Secret     string
	Issuer     string
	ExpireHour int
}

type TelemetryConfig struct {
	Enabled      bool   // OTEL_ENABLED (default: false)
	CollectorURL string // OTEL_COLLECTOR_URL (default: localhost:4318)
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvAsInt("SERVER_PORT", 8080),
			Env:  getEnv("SERVER_ENV", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "kensan"),
			Password: getEnv("DB_PASSWORD", "kensan"),
			DBName:   getEnv("DB_NAME", "kensan"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			Issuer:     getEnv("JWT_ISSUER", "kensan"),
			ExpireHour: getEnvAsInt("JWT_EXPIRE_HOUR", 720),
		},
		Telemetry: TelemetryConfig{
			Enabled:      getEnvAsBool("OTEL_ENABLED", false),
			CollectorURL: getEnv("OTEL_COLLECTOR_URL", "localhost:4318"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
