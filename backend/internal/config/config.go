package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Env       string        `env:"APP_ENV" envDefault:"development"`
	HTTP      HTTP          `envPrefix:""`
	DB        Database      `envPrefix:"DB_"`
	JWT       JWT           `envPrefix:"JWT_"`
	Bcrypt    Bcrypt        `envPrefix:"BCRYPT_"`
	RateLimit RateLimit     `envPrefix:"RATE_LIMIT_"`
	CORS      CORS          `envPrefix:"CORS_"`
	Hass      Hass          `envPrefix:"HASS_"`
	Paths     Paths
	// MetricsToken, when non-empty, requires every request to /metrics to carry
	// the header `X-Metrics-Token: <value>`. When unset (default), /metrics is
	// open — acceptable for local dev but should be set in any internet-facing
	// deployment (see README §"Local observability").
	MetricsToken    string        `env:"METRICS_TOKEN"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

type HTTP struct {
	Addr string `env:"HTTP_ADDR" envDefault:":8080"`
}

type Database struct {
	Host     string `env:"HOST" envDefault:"localhost"`
	Port     int    `env:"PORT" envDefault:"5432"`
	User     string `env:"USER" envDefault:"smartclass"`
	Password string `env:"PASSWORD" envDefault:"smartclass"`
	Name     string `env:"NAME" envDefault:"smartclass"`
	SSLMode  string `env:"SSLMODE" envDefault:"require"`
}

func (d Database) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode)
}

type JWT struct {
	Secret     string        `env:"SECRET,required"`
	AccessTTL  time.Duration `env:"ACCESS_TTL" envDefault:"15m"`
	RefreshTTL time.Duration `env:"REFRESH_TTL" envDefault:"720h"`
	Issuer     string        `env:"ISSUER" envDefault:"smartclass"`
}

type Bcrypt struct {
	Cost int `env:"COST" envDefault:"10"`
}

type RateLimit struct {
	RPS   int `env:"RPS" envDefault:"50"`
	Burst int `env:"BURST" envDefault:"100"`
	// TrustedProxies is an optional comma-separated list of CIDR prefixes that
	// are trusted as reverse proxies for X-Forwarded-For. When empty, any
	// loopback or RFC-1918 address is trusted (backward-compatible default).
	// Example: RATE_LIMIT_TRUSTED_PROXIES=10.0.0.0/8,172.16.0.0/12
	TrustedProxies []string `env:"TRUSTED_PROXIES" envSeparator:","`
}

type CORS struct {
	// Default to localhost dev origins; production must set CORS_ORIGINS explicitly.
	Origins []string `env:"ORIGINS" envSeparator:"," envDefault:"http://localhost:5173,http://localhost:3000,http://localhost:4200"`
}

type Hass struct {
	URL           string `env:"URL" envDefault:"http://homeassistant:8123"`
	OwnerName     string `env:"OWNER_NAME" envDefault:"Smart Classroom"`
	OwnerUsername string `env:"OWNER_USERNAME" envDefault:"smartclass"`
	// No default — must be set explicitly to prevent accidental well-known credentials.
	OwnerPassword string `env:"OWNER_PASSWORD"`
	Language      string `env:"LANGUAGE" envDefault:"kz"`
}

type Paths struct {
	Migrations string `env:"MIGRATIONS_DIR" envDefault:"./migrations"`
	I18n       string `env:"I18N_DIR" envDefault:"./locales"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if len(cfg.JWT.Secret) < 32 {
		return Config{}, fmt.Errorf("config: JWT_SECRET must be at least 32 characters")
	}
	if cfg.Env == "production" {
		if cfg.DB.SSLMode == "disable" || cfg.DB.SSLMode == "allow" {
			return Config{}, fmt.Errorf("config: DB_SSLMODE=%q is not safe for production; use require, verify-ca, or verify-full", cfg.DB.SSLMode)
		}
		if len(cfg.CORS.Origins) == 0 {
			return Config{}, fmt.Errorf("config: CORS_ORIGINS must be set explicitly in production")
		}
		for _, o := range cfg.CORS.Origins {
			if o == "" || o == "*" ||
				strings.HasPrefix(o, "http://localhost") ||
				strings.HasPrefix(o, "http://127.0.0.1") {
				return Config{}, fmt.Errorf("config: CORS_ORIGINS=%q is not safe for production", cfg.CORS.Origins)
			}
		}
	}
	return cfg, nil
}
