package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv                    string
	Port                      string
	LogLevel                  string
	DatabaseURL               string
	JWTSecret                 string
	JWTAccessExpiry           time.Duration
	RefreshTokenExpiry        time.Duration
	CookieSecure              bool
	Argon2MemoryKiB           uint32
	Argon2Iterations          uint32
	Argon2Parallelism         uint8
	CORSAllowedOrigins        []string
	OperatingHoursStart       string
	OperatingHoursEnd         string
	BookingMinDurationMinutes int
	BookingMaxDurationHours   int
	BookingMaxFutureDays      int
	UploadDir                 string
	UploadMaxBytes            int64
	SSEHeartbeat              time.Duration
}

// Load loads configuration from environment variables with optional .env file support.
func Load() (*Config, error) {
	// Best-effort load from .env in current or parent directory
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	cfg := &Config{
		AppEnv:                    getEnv("APP_ENV", "development"),
		Port:                      getEnv("PORT", "8080"),
		LogLevel:                  getEnv("LOG_LEVEL", "info"),
		DatabaseURL:               getEnv("DATABASE_URL", ""),
		JWTSecret:                 getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:           getDurationEnv("JWT_ACCESS_EXPIRY", 15*time.Minute),
		RefreshTokenExpiry:        getDurationEnv("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
		CookieSecure:              getBoolEnv("COOKIE_SECURE", false),
		Argon2MemoryKiB:           uint32(getIntEnv("ARGON2_MEMORY_KIB", 19*1024)),
		Argon2Iterations:          uint32(getIntEnv("ARGON2_ITERATIONS", 2)),
		Argon2Parallelism:         uint8(getIntEnv("ARGON2_PARALLELISM", 1)),
		CORSAllowedOrigins:        getStringSliceEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173"}),
		OperatingHoursStart:       getEnv("OPERATING_HOURS_START", "07:00"),
		OperatingHoursEnd:         getEnv("OPERATING_HOURS_END", "21:00"),
		BookingMinDurationMinutes: getIntEnv("BOOKING_MIN_DURATION_MINUTES", 30),
		BookingMaxDurationHours:   getIntEnv("BOOKING_MAX_DURATION_HOURS", 8),
		BookingMaxFutureDays:      getIntEnv("BOOKING_MAX_FUTURE_DAYS", 30),
		UploadDir:                 getEnv("UPLOAD_DIR", "./uploads"),
		UploadMaxBytes:            getInt64Env("UPLOAD_MAX_BYTES", 5*1024*1024),
		SSEHeartbeat:              getDurationEnv("SSE_HEARTBEAT", 25*time.Second),
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

	if cfg.UploadDir == "" || cfg.UploadMaxBytes < 1 {
		return nil, fmt.Errorf("UPLOAD_DIR must be set and UPLOAD_MAX_BYTES must be positive")
	}
	if cfg.SSEHeartbeat < time.Second {
		return nil, fmt.Errorf("SSE_HEARTBEAT must be at least 1s")
	}
	if _, err := cfg.BookingRules(); err != nil {
		return nil, err
	}

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

// BookingRules are the BR-01 limits, parsed from the environment settings.
type BookingRules struct {
	OpenMinute    int // minutes after midnight WIB when bookings may start
	CloseMinute   int // minutes after midnight WIB when bookings must end
	MinDuration   time.Duration
	MaxDuration   time.Duration
	MaxFutureDays int
}

// BookingRules parses and validates the booking settings. Load calls it so a bad value stops the
// server at start instead of rejecting every booking later.
func (c *Config) BookingRules() (BookingRules, error) {
	open, err := parseClock(c.OperatingHoursStart)
	if err != nil {
		return BookingRules{}, fmt.Errorf("OPERATING_HOURS_START: %w", err)
	}
	closing, err := parseClock(c.OperatingHoursEnd)
	if err != nil {
		return BookingRules{}, fmt.Errorf("OPERATING_HOURS_END: %w", err)
	}
	if open >= closing {
		return BookingRules{}, fmt.Errorf("OPERATING_HOURS_START must be before OPERATING_HOURS_END")
	}
	if c.BookingMinDurationMinutes < 1 || c.BookingMaxDurationHours < 1 ||
		time.Duration(c.BookingMinDurationMinutes)*time.Minute > time.Duration(c.BookingMaxDurationHours)*time.Hour {
		return BookingRules{}, fmt.Errorf("BOOKING_MIN_DURATION_MINUTES and BOOKING_MAX_DURATION_HOURS must be positive with min <= max")
	}
	if c.BookingMaxFutureDays < 1 {
		return BookingRules{}, fmt.Errorf("BOOKING_MAX_FUTURE_DAYS must be at least 1")
	}
	return BookingRules{
		OpenMinute:    open,
		CloseMinute:   closing,
		MinDuration:   time.Duration(c.BookingMinDurationMinutes) * time.Minute,
		MaxDuration:   time.Duration(c.BookingMaxDurationHours) * time.Hour,
		MaxFutureDays: c.BookingMaxFutureDays,
	}, nil
}

// parseClock reads "HH:MM" as minutes after midnight; "24:00" is allowed as a closing time.
func parseClock(v string) (int, error) {
	hh, mm, ok := strings.Cut(v, ":")
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if !ok || errH != nil || errM != nil || h < 0 || h > 24 || m < 0 || m > 59 || (h == 24 && m != 0) {
		return 0, fmt.Errorf("%q is not a valid HH:MM time", v)
	}
	return h*60 + m, nil
}
