package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Upstream UpstreamConfig
	Logger   LoggerConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type UpstreamConfig struct {
	URL     string
	Timeout time.Duration
}

type LoggerConfig struct {
	Level  string
	Format string
}

func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("SERVER_HOST", "0.0.0.0")
	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("UPSTREAM_URL", "http://localhost:8085")
	v.SetDefault("UPSTREAM_TIMEOUT", "30s")
	v.SetDefault("LOGGER_LEVEL", "info")
	v.SetDefault("LOGGER_FORMAT", "json")

	// Env
	v.AutomaticEnv()

	timeout, err := time.ParseDuration(v.GetString("UPSTREAM_TIMEOUT"))
	if err != nil {
		timeout = 30 * time.Second
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: v.GetString("SERVER_HOST"),
			Port: v.GetInt("SERVER_PORT"),
		},
		Upstream: UpstreamConfig{
			URL:     v.GetString("UPSTREAM_URL"),
			Timeout: timeout,
		},
		Logger: LoggerConfig{
			Level:  v.GetString("LOGGER_LEVEL"),
			Format: v.GetString("LOGGER_FORMAT"),
		},
	}

	return cfg, nil
}
