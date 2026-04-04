package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Log      LogConfig
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port         string
	ReadTimeout  int
	WriteTimeout int
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string
	Format string
}

// Load reads configuration from environment variables
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getEnvAsInt("READ_TIMEOUT", 30),
			WriteTimeout: getEnvAsInt("WRITE_TIMEOUT", 30),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
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