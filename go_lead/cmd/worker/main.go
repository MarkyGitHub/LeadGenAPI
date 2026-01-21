package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/checkfox/go_lead/internal/client"
	"github.com/checkfox/go_lead/internal/config"
	"github.com/checkfox/go_lead/internal/database"
	"github.com/checkfox/go_lead/internal/logger"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/checkfox/go_lead/internal/repository"
	"github.com/checkfox/go_lead/internal/services"
	"github.com/checkfox/go_lead/internal/worker"
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

	logger.Info(ctx, "Worker starting",
		"poll_interval", cfg.Worker.PollInterval,
		"concurrency", cfg.Worker.Concurrency,
		"max_retry_attempts", cfg.Retry.MaxAttempts)

	// Initialize database connection
	dbWrapper, err := database.InitFromConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbWrapper.Close()

	logger.Info(ctx, "Database connection established")

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

	// Initialize services
	validator := services.NewValidator()
	normalizer := services.NewNormalizer()
	mapper := services.NewMapper(cfg)

	// Initialize Customer API client
	customerAPIClient := client.NewCustomerAPIClient(
		cfg.CustomerAPI.URL,
		cfg.CustomerAPI.Token,
		cfg.CustomerAPI.Timeout,
	)

	// Calculate exponential backoff delays based on configuration
	exponentialBackoffDelays := make([]time.Duration, cfg.Retry.MaxAttempts)
	for i := 0; i < cfg.Retry.MaxAttempts; i++ {
		// Exponential backoff: base * 2^i
		exponentialBackoffDelays[i] = cfg.Retry.BackoffBase * time.Duration(1<<uint(i))
	}

	logger.Info(ctx, "Retry configuration",
		"max_attempts", cfg.Retry.MaxAttempts,
		"backoff_base", cfg.Retry.BackoffBase,
		"backoff_delays", exponentialBackoffDelays)

	// Create worker processor
	processor := worker.NewProcessor(worker.ProcessorConfig{
		Queue:                    jobQueue,
		LeadRepo:                 leadRepo,
		DeliveryAttemptRepo:      deliveryAttemptRepo,
		Validator:                validator,
		Normalizer:               normalizer,
		Mapper:                   mapper,
		CustomerAPIClient:        customerAPIClient,
		PollInterval:             cfg.Worker.PollInterval,
		MaxDeliveryAttempts:      cfg.Retry.MaxAttempts,
		ExponentialBackoffDelays: exponentialBackoffDelays,
	})

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create context for worker
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start worker in a goroutine
	workerErrors := make(chan error, 1)
	go func() {
		workerErrors <- processor.Start(workerCtx)
	}()

	logger.Info(ctx, "Worker started successfully")

	// Wait for shutdown signal or worker error
	select {
	case err := <-workerErrors:
		if err != nil && err != context.Canceled {
			logger.Error(ctx, "Worker error", "error", err.Error())
		}

	case sig := <-sigChan:
		logger.Info(ctx, "Received shutdown signal", "signal", sig.String())

		// Cancel worker context to trigger graceful shutdown
		cancel()

		// Wait for worker to finish with timeout
		shutdownTimeout := time.NewTimer(30 * time.Second)
		defer shutdownTimeout.Stop()

		select {
		case <-workerErrors:
			logger.Info(ctx, "Worker stopped gracefully")
		case <-shutdownTimeout.C:
			logger.Warn(ctx, "Worker shutdown timeout exceeded, forcing exit")
		}
	}

	logger.Info(ctx, "Worker shutdown complete")
}
