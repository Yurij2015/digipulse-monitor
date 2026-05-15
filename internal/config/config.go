package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server       ServerConfig
	Log          LogConfig
	Redis        RedisConfig
	Connectivity ConnectivityConfig
}

// ConnectivityConfig controls pre-flight checks before dequeuing monitor tasks.
type ConnectivityConfig struct {
	InternetCheckEnabled bool
	InternetProbeURL     string
	ProbeTimeout         time.Duration
	OfflineWait          time.Duration
	RedisBRPopBlock      time.Duration
}

type RedisConfig struct {
	Addr           string
	Password       string
	DB             int
	ChannelName    string
	ResultsChannel string
	HeartbeatKey   string
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
	internetCheckEnabled := getEnvBool("INTERNET_CHECK_ENABLED", true)
	redisBRPopBlock := time.Duration(getEnvAsInt("REDIS_BRPOP_TIMEOUT_SEC", 30)) * time.Second
	if internetCheckEnabled && redisBRPopBlock <= 0 {
		redisBRPopBlock = 30 * time.Second
	}

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
			Addr:           getEnv("REDIS_ADDR", "localhost:6379"),
			Password:       getEnv("REDIS_PASSWORD", ""),
			DB:             getEnvAsInt("REDIS_DB", 0),
			ChannelName:    getEnv("MONITOR_REDIS_CHANNEL", "monitoring:tasks"),
			ResultsChannel: getEnv("MONITOR_RESULTS_CHANNEL", "monitoring:results"),
			HeartbeatKey:   getEnv("MONITOR_HEARTBEAT_KEY", "go_monitor:last_heartbeat"),
		},
		Connectivity: ConnectivityConfig{
			InternetCheckEnabled: internetCheckEnabled,
			InternetProbeURL:     getEnv("INTERNET_PROBE_URL", "https://www.cloudflare.com"),
			ProbeTimeout:         time.Duration(getEnvAsInt("INTERNET_PROBE_TIMEOUT_SEC", 5)) * time.Second,
			OfflineWait:          time.Duration(getEnvAsInt("INTERNET_OFFLINE_WAIT_SEC", 10)) * time.Second,
			RedisBRPopBlock:      redisBRPopBlock,
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

func getEnvBool(key string, defaultValue bool) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}
