package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/streamhive/video-catalog-api/internal/api"
	"github.com/streamhive/video-catalog-api/internal/db"
	"github.com/streamhive/video-catalog-api/internal/queue"
	"github.com/streamhive/video-catalog-api/internal/services"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	// Initialize database
	database, err := db.NewConnection()
	if err != nil {
		sugar.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := db.RunMigrations(database); err != nil {
		sugar.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize services
	videoService := services.NewVideoService(database, sugar)

	// Initialize RabbitMQ consumer
	consumer, err := queue.NewConsumer(sugar)
	if err != nil {
		sugar.Fatalf("Failed to initialize RabbitMQ consumer: %v", err)
	}
	defer consumer.Close()

	// Start RabbitMQ consumer
	go func() {
		if err := consumer.StartConsuming(videoService); err != nil {
			sugar.Errorf("RabbitMQ consumer error: %v", err)
		}
	}()

	// Initialize Gin router
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-User-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	api.SetupRoutes(router, videoService, sugar)

	// Get port from environment or use default
	port := getEnv("PORT", "8080")

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		sugar.Infow("Starting server", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	sugar.Info("Shutting down server...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		sugar.Fatalf("Server forced to shutdown: %v", err)
	}

	sugar.Info("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
