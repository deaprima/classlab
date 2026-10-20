package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv             string
	Port               string
	LogLevel           string
	DatabaseURL        string
	JWTSecret          string
	JWTAccessExpiry    time.Duration
	RefreshTokenExpiry time.Duration
	CookieSecure       bool
	Argon2MemoryKiB    uint32
	Argon2Iterations   uint32
	Argon2Parallelism  uint8
	CORSAllowedOrigins []string
	UploadDir          string
	SSEHeartbeat       time.Duration
	DBLockTimeout      time.Duration

	// Business holds the BR-11 to BR-28 settings. The older booking and upload settings above stay until
	// the phase that replaces the code reading them (bookings: Phase 3, uploads: Phase 3).
	Business Business
}

// Load loads configuration from environment variables with optional .env file support.
func Load() (*Config, error) {
	// Best-effort load from .env in current or parent directory
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	cfg := &Config{
		AppEnv:             getEnv("APP_ENV", "development"),
		Port:               getEnv("PORT", "8080"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:    getDurationEnv("JWT_ACCESS_EXPIRY", 15*time.Minute),
		RefreshTokenExpiry: getDurationEnv("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
		CookieSecure:       getBoolEnv("COOKIE_SECURE", false),
		Argon2MemoryKiB:    uint32(getIntEnv("ARGON2_MEMORY_KIB", 19*1024)),
		Argon2Iterations:   uint32(getIntEnv("ARGON2_ITERATIONS", 2)),
		Argon2Parallelism:  uint8(getIntEnv("ARGON2_PARALLELISM", 1)),
		CORSAllowedOrigins: getStringSliceEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173"}),
		UploadDir:          getEnv("UPLOAD_DIR", "./uploads"),
		SSEHeartbeat:       getDurationEnv("SSE_HEARTBEAT", 25*time.Second),
		DBLockTimeout:      getDurationEnv("DB_LOCK_TIMEOUT", 5*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be specified")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET must be specified")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	if cfg.Argon2MemoryKiB < 8 || cfg.Argon2Iterations < 1 || cfg.Argon2Parallelism < 1 {
		return nil, fmt.Errorf("ARGON2_* parameters are out of range")
	}
	// The refresh cookie carries the session; without Secure it would travel over plain HTTP.
	if cfg.AppEnv == "production" && !cfg.CookieSecure {
		return nil, fmt.Errorf("COOKIE_SECURE must be true when APP_ENV=production")
	}

	if cfg.UploadDir == "" {
		return nil, fmt.Errorf("UPLOAD_DIR must be set")
	}
	if cfg.SSEHeartbeat < time.Second {
		return nil, fmt.Errorf("SSE_HEARTBEAT must be at least 1s")
	}
	if cfg.DBLockTimeout < 100*time.Millisecond {
		return nil, fmt.Errorf("DB_LOCK_TIMEOUT must be at least 100ms")
	}
	if err := validateCORSOrigins(cfg.CORSAllowedOrigins); err != nil {
		return nil, err
	}
	business, err := loadBusiness(os.Getenv)
	if err != nil {
		return nil, fmt.Errorf("invalid business settings: %w", err)
	}
	cfg.Business = business

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		parsed, err := strconv.ParseBool(val)
		if err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		parsed, err := strconv.Atoi(val)
		if err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getInt64Env(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		parsed, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		// time.ParseDuration has no day unit, but .env.example documents "7d".
		if days, ok := strings.CutSuffix(val, "d"); ok {
			if n, err := strconv.Atoi(days); err == nil {
				return time.Duration(n) * 24 * time.Hour
			}
		}
		parsed, err := time.ParseDuration(val)
		if err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getStringSliceEnv(key string, defaultVal []string) []string {
	if val := os.Getenv(key); val != "" {
		parts := strings.Split(val, ",")
		var cleaned []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		if len(cleaned) > 0 {
			return cleaned
		}
	}
	return defaultVal
}

// validateCORSOrigins refuses a wildcard: the API always answers with credentials allowed, and a
// browser rejects "*" there, while a server that reflects the caller's origin instead would let any
// site read authenticated responses. Each entry must be a bare origin (scheme and host, no path).
func validateCORSOrigins(origins []string) error {
	for _, o := range origins {
		if o == "*" {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS must list explicit origins; \"*\" is not allowed because credentials are enabled")
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS entry %q must look like https://host[:port] with no path", o)
		}
	}
	return nil
}
