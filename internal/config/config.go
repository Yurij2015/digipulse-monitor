package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	Server  ServerConfig
	Log     LogConfig
	Redis   RedisConfig
	Backend BackendConfig
}

type RedisConfig struct {
	Addr        string
	Password    string
	DB          int
	ChannelName string
}

type BackendConfig struct {
	BaseURL string
	Key     string
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port         string
	ReadTimeout  int
	WriteTimeout int
}

// LogConfig holds the logging configuration
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
		Redis: RedisConfig{
			Addr:        getEnv("REDIS_ADDR", "localhost:6379"),
			Password:    getEnv("REDIS_PASSWORD", ""),
			DB:          getEnvAsInt("REDIS_DB", 0),
			ChannelName: getEnv("MONITOR_REDIS_CHANNEL", "monitoring:tasks"),
		},
		Backend: BackendConfig{
			BaseURL: getEnv("BACKEND_URL", "http://localhost:8000/api/internal"),
			Key:     getEnv("MONITOR_API_KEY", ""),
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
