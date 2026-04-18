package config

import (
	"fmt"
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
	SSLMode  string `env:"SSLMODE" envDefault:"disable"`
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
}

type CORS struct {
	Origins []string `env:"ORIGINS" envSeparator:"," envDefault:"*"`
}

type Hass struct {
	URL           string `env:"URL" envDefault:"http://homeassistant:8123"`
	OwnerName     string `env:"OWNER_NAME" envDefault:"Smart Classroom"`
	OwnerUsername string `env:"OWNER_USERNAME" envDefault:"smartclass"`
	OwnerPassword string `env:"OWNER_PASSWORD" envDefault:"smartclass1234"`
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
	if len(cfg.JWT.Secret) < 16 {
		return Config{}, fmt.Errorf("config: JWT_SECRET must be at least 16 bytes")
	}
	return cfg, nil
}
