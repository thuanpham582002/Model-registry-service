package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"model-registry-service/internal/adapters/primary/http/handlers"
	"model-registry-service/internal/adapters/primary/http/middleware"
	"model-registry-service/internal/adapters/secondary/aigateway"
	"model-registry-service/internal/adapters/secondary/kserve"
	"model-registry-service/internal/adapters/secondary/postgres"
	"model-registry-service/internal/adapters/secondary/prometheus"
	"model-registry-service/internal/config"
	output "model-registry-service/internal/core/ports/output"
	"model-registry-service/internal/core/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	initLogger(cfg)

	// Create database pool
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("parse db config: %v", err)
	}
	poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.Database.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.Database.ConnMaxLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		log.Fatalf("create db pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	log.Info("database connection established")

	// ============================================================================
	// Hexagonal Architecture Wiring
	// ============================================================================

	// Secondary Adapters (Output Ports - Repositories)
	modelRepo := postgres.NewRegisteredModelRepository(pool)
	versionRepo := postgres.NewModelVersionRepository(pool)
	servingEnvRepo := postgres.NewServingEnvironmentRepository(pool)
	isvcRepo := postgres.NewInferenceServiceRepository(pool)
	serveModelRepo := postgres.NewServeModelRepository(pool)
	trafficConfigRepo := postgres.NewTrafficConfigRepository(pool)
	trafficVariantRepo := postgres.NewTrafficVariantRepository(pool)
	virtualModelRepo := postgres.NewVirtualModelRepository(pool)

	// KServe Client (Optional - based on config)
	var kserveClient output.KServeClient
	if cfg.Kubernetes.Enabled {
		client, err := kserve.NewKServeClient(&cfg.Kubernetes)
		if err != nil {
			log.Warnf("KServe client init failed (continuing without K8s integration): %v", err)
		} else {
			kserveClient = client
			log.Info("KServe client initialized")
		}
	} else {
		log.Info("KServe integration disabled")
	}

	// AI Gateway Client (Optional - based on config)
	var aiGatewayClient output.AIGatewayClient
	if cfg.AIGateway.Enabled {
		client, err := aigateway.NewAIGatewayClient(&cfg.AIGateway)
		if err != nil {
			log.Warnf("AI Gateway client init failed (continuing without AI Gateway integration): %v", err)
		} else {
			aiGatewayClient = client
			log.Info("AI Gateway client initialized")
		}
	} else {
		log.Info("AI Gateway integration disabled")
	}

	// Prometheus Client (Optional - based on config)
	var prometheusClient output.PrometheusClient
	if cfg.Prometheus.Enabled {
		prometheusClient = prometheus.NewPrometheusClient(&cfg.Prometheus)
		log.Info("Prometheus client initialized")
	} else {
		log.Info("Prometheus integration disabled")
	}

	// Core Services (Application Layer)
	modelSvc := services.NewRegisteredModelService(modelRepo)
	versionSvc := services.NewModelVersionService(versionRepo, modelRepo)
	artifactSvc := services.NewModelArtifactService(versionRepo, modelRepo)
	servingEnvSvc := services.NewServingEnvironmentService(servingEnvRepo, isvcRepo)
	isvcSvc := services.NewInferenceServiceService(isvcRepo, servingEnvRepo, modelRepo, versionRepo, kserveClient)
	serveModelSvc := services.NewServeModelService(serveModelRepo, isvcRepo, versionRepo)
	deploySvc := services.NewDeployService(servingEnvRepo, isvcRepo, serveModelRepo, modelRepo, versionRepo, kserveClient)
	trafficSvc := services.NewTrafficService(trafficConfigRepo, trafficVariantRepo, isvcRepo, versionRepo, servingEnvRepo, kserveClient, aiGatewayClient)
	virtualModelSvc := services.NewVirtualModelService(virtualModelRepo, aiGatewayClient)
	metricsSvc := services.NewMetricsService(prometheusClient, isvcRepo)
	aiBackendSvc := services.NewAIServiceBackendService(aiGatewayClient)
	backendSvc := services.NewBackendService(aiGatewayClient)

	// Primary Adapter (HTTP Handlers)
	h := handlers.New(modelSvc, versionSvc, artifactSvc, servingEnvSvc, isvcSvc, serveModelSvc, deploySvc, trafficSvc, virtualModelSvc, metricsSvc, aiBackendSvc, backendSvc)

	// Setup router
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.Logging(), gin.Recovery())

	api := router.Group("/api/v1/model-registry")
	h.RegisterRoutes(api)

	// Health check with DB ping
	router.GET("/healthz", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		log.Infof("starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced shutdown: %v", err)
	}

	log.Info("server stopped")
}

func initLogger(cfg *config.Config) {
	level, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	if cfg.Logger.Format == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	}
}
