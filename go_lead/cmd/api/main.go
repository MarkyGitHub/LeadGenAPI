package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/checkfox/go_lead/internal/config"
	"github.com/checkfox/go_lead/internal/database"
	"github.com/checkfox/go_lead/internal/handlers"
	"github.com/checkfox/go_lead/internal/logger"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/checkfox/go_lead/internal/repository"
)

func main() {
	// Initialize structured logger
	logger.Init()
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Info(ctx, "API Server starting",
		"host", cfg.API.Host,
		"port", cfg.API.Port,
		"auth_enabled", cfg.Auth.Enabled)

	// Initialize database connection
	dbWrapper, err := database.InitFromConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbWrapper.Close()

	logger.Info(ctx, "Database connection established")

	// Run database migrations
	if err := database.RunMigrations(dbWrapper, "./migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	logger.Info(ctx, "Database migrations completed")

	// Initialize queue client
	jobQueue, err := queue.NewDBQueue(dbWrapper.DB)
	if err != nil {
		log.Fatalf("Failed to initialize queue: %v", err)
	}
	defer jobQueue.Close()

	logger.Info(ctx, "Queue initialized")

	// Initialize repositories
	leadRepo := repository.NewLeadRepository(dbWrapper.DB)
	deliveryAttemptRepo := repository.NewDeliveryAttemptRepository(dbWrapper.DB)

	// Initialize handlers
	webhookHandler := handlers.NewWebhookHandler(leadRepo, jobQueue)
	statsHandler := handlers.NewStatsHandler(leadRepo, deliveryAttemptRepo)

	// Initialize middleware
	authMiddleware := handlers.NewAuthMiddleware(cfg)
	recoveryMiddleware := handlers.NewRecoveryMiddleware()

	// Set up HTTP routes
	mux := http.NewServeMux()

	// Webhook endpoint with authentication and recovery middleware
	mux.HandleFunc("/webhooks/leads",
		recoveryMiddleware.Recover(
			authMiddleware.Authenticate(
				webhookHandler.HandleLeadWebhook)))

	// Stats endpoints
	mux.HandleFunc("/stats/leads/counts",
		recoveryMiddleware.Recover(statsHandler.HandleLeadCountsByStatus))
	mux.HandleFunc("/stats/leads/recent",
		recoveryMiddleware.Recover(statsHandler.HandleRecentLeads))
	mux.HandleFunc("/stats/leads/", // Handles /stats/leads/{id}/history
		recoveryMiddleware.Recover(statsHandler.HandleLeadHistory))

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create HTTP server
	addr := fmt.Sprintf("%s:%s", cfg.API.Host, cfg.API.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info(ctx, "HTTP server listening", "address", addr)
		serverErrors <- server.ListenAndServe()
	}()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrors:
		log.Fatalf("Server error: %v", err)

	case sig := <-sigChan:
		logger.Info(ctx, "Received shutdown signal", "signal", sig.String())

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error(ctx, "Server shutdown error", "error", err.Error())
			// Force close if graceful shutdown fails
			server.Close()
		}

		logger.Info(ctx, "Server shutdown complete")
	}
}
