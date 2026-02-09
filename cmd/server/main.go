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
	"model-registry-service/internal/proxy"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Init logger
	initLogger(cfg)

	// Create proxy client
	proxyClient := proxy.NewClient(cfg.Upstream.URL, cfg.Upstream.Timeout)

	// Create handler
	h := handler.New(proxyClient)

	// Setup router
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.Logging(), gin.Recovery())

	api := router.Group("/api/v1/model-registry")
	h.RegisterRoutes(api)

	// Health check
	router.GET("/healthz", func(c *gin.Context) {
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
