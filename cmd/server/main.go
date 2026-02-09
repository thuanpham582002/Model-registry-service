package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"model-registry-service/internal/config"
	"model-registry-service/internal/handler"
	"model-registry-service/internal/middleware"
	"model-registry-service/internal/repository"
	"model-registry-service/internal/usecase"

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

	// Create layers
	modelRepo := repository.NewRegisteredModelRepository(pool)
	versionRepo := repository.NewModelVersionRepository(pool)

	modelUC := usecase.NewRegisteredModelUseCase(modelRepo)
	versionUC := usecase.NewModelVersionUseCase(versionRepo, modelRepo)
	artifactUC := usecase.NewModelArtifactUseCase(versionRepo, modelRepo)

	h := handler.New(modelUC, versionUC, artifactUC)

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
