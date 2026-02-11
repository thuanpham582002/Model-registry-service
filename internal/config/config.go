package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Logger     LoggerConfig
	Kubernetes KubernetesConfig
	AIGateway  AIGatewayConfig
	Prometheus PrometheusConfig
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

type KubernetesConfig struct {
	Enabled        bool
	InCluster      bool
	KubeConfigPath string
	DefaultNS      string
}

type AIGatewayConfig struct {
	Enabled          bool
	InCluster        bool
	KubeConfigPath   string
	DefaultNamespace string
	GatewayName      string
	GatewayNamespace string
}

type PrometheusConfig struct {
	Enabled bool
	URL     string
	Timeout time.Duration
}

func Load() (*Config, error) {
	v := viper.New()

	// Server defaults
	v.SetDefault("SERVER_HOST", "0.0.0.0")
	v.SetDefault("SERVER_PORT", 8080)

	// Database defaults
	v.SetDefault("DATABASE_HOST", "localhost")
	v.SetDefault("DATABASE_PORT", 5432)
	v.SetDefault("DATABASE_USER", "postgres")
	v.SetDefault("DATABASE_PASSWORD", "postgres")
	v.SetDefault("DATABASE_DBNAME", "model_registry")
	v.SetDefault("DATABASE_SSLMODE", "disable")
	v.SetDefault("DATABASE_MAX_OPEN_CONNS", 25)
	v.SetDefault("DATABASE_MAX_IDLE_CONNS", 5)
	v.SetDefault("DATABASE_CONN_MAX_LIFETIME", "5m")

	// Logger defaults
	v.SetDefault("LOGGER_LEVEL", "info")
	v.SetDefault("LOGGER_FORMAT", "json")

	// Kubernetes/KServe defaults
	v.SetDefault("KSERVE_ENABLED", false)
	v.SetDefault("KUBERNETES_IN_CLUSTER", false)
	v.SetDefault("KUBERNETES_KUBECONFIG", "")
	v.SetDefault("KUBERNETES_DEFAULT_NAMESPACE", "model-serving")

	// AI Gateway defaults
	v.SetDefault("AIGATEWAY_ENABLED", false)
	v.SetDefault("AIGATEWAY_IN_CLUSTER", false)
	v.SetDefault("AIGATEWAY_KUBECONFIG", "")
	v.SetDefault("AIGATEWAY_DEFAULT_NAMESPACE", "model-serving")
	v.SetDefault("AIGATEWAY_GATEWAY_NAME", "ai-gateway")
	v.SetDefault("AIGATEWAY_GATEWAY_NAMESPACE", "envoy-gateway-system")

	// Prometheus defaults
	v.SetDefault("PROMETHEUS_ENABLED", false)
	v.SetDefault("PROMETHEUS_URL", "http://prometheus:9090")
	v.SetDefault("PROMETHEUS_TIMEOUT", "30s")

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
		Kubernetes: KubernetesConfig{
			Enabled:        v.GetBool("KSERVE_ENABLED"),
			InCluster:      v.GetBool("KUBERNETES_IN_CLUSTER"),
			KubeConfigPath: v.GetString("KUBERNETES_KUBECONFIG"),
			DefaultNS:      v.GetString("KUBERNETES_DEFAULT_NAMESPACE"),
		},
		AIGateway: AIGatewayConfig{
			Enabled:          v.GetBool("AIGATEWAY_ENABLED"),
			InCluster:        v.GetBool("AIGATEWAY_IN_CLUSTER"),
			KubeConfigPath:   v.GetString("AIGATEWAY_KUBECONFIG"),
			DefaultNamespace: v.GetString("AIGATEWAY_DEFAULT_NAMESPACE"),
			GatewayName:      v.GetString("AIGATEWAY_GATEWAY_NAME"),
			GatewayNamespace: v.GetString("AIGATEWAY_GATEWAY_NAMESPACE"),
		},
		Prometheus: PrometheusConfig{
			Enabled: v.GetBool("PROMETHEUS_ENABLED"),
			URL:     v.GetString("PROMETHEUS_URL"),
			Timeout: v.GetDuration("PROMETHEUS_TIMEOUT"),
		},
	}

	return cfg, nil
}
