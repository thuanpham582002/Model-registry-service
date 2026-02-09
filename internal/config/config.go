package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Logger   LoggerConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
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
	v.SetDefault("DATABASE_HOST", "localhost")
	v.SetDefault("DATABASE_PORT", 5432)
	v.SetDefault("DATABASE_USER", "postgres")
	v.SetDefault("DATABASE_PASSWORD", "postgres")
	v.SetDefault("DATABASE_DBNAME", "cmp")
	v.SetDefault("DATABASE_SSLMODE", "disable")
	v.SetDefault("DATABASE_MAX_OPEN_CONNS", 25)
	v.SetDefault("DATABASE_MAX_IDLE_CONNS", 5)
	v.SetDefault("DATABASE_CONN_MAX_LIFETIME", "5m")
	v.SetDefault("LOGGER_LEVEL", "info")
	v.SetDefault("LOGGER_FORMAT", "json")

	v.AutomaticEnv()

	connMaxLifetime, err := time.ParseDuration(v.GetString("DATABASE_CONN_MAX_LIFETIME"))
	if err != nil {
		connMaxLifetime = 5 * time.Minute
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: v.GetString("SERVER_HOST"),
			Port: v.GetInt("SERVER_PORT"),
		},
		Database: DatabaseConfig{
			Host:            v.GetString("DATABASE_HOST"),
			Port:            v.GetInt("DATABASE_PORT"),
			User:            v.GetString("DATABASE_USER"),
			Password:        v.GetString("DATABASE_PASSWORD"),
			DBName:          v.GetString("DATABASE_DBNAME"),
			SSLMode:         v.GetString("DATABASE_SSLMODE"),
			MaxOpenConns:    v.GetInt("DATABASE_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("DATABASE_MAX_IDLE_CONNS"),
			ConnMaxLifetime: connMaxLifetime,
		},
		Logger: LoggerConfig{
			Level:  v.GetString("LOGGER_LEVEL"),
			Format: v.GetString("LOGGER_FORMAT"),
		},
	}

	return cfg, nil
}
