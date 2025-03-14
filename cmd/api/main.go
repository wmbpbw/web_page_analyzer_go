package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"log/slog"
	"webPageAnalyzerGO/internal/api"
	"webPageAnalyzerGO/internal/config"
	"webPageAnalyzerGO/internal/repository"
)

func main() {
	logger := setupLogger()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Info("No .env file found, using environment variables")
	}

	// Create config
	cfg, err := config.New()
	if err != nil {
		logger.Error("Failed to create config", "error", err)
		os.Exit(1)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Initialize MongoDB connection
	mongoRepo, err := repository.NewMongoRepository(ctx, cfg.MongoDB)
	if err != nil {
		logger.Error("Failed to create MongoDB repository", "error", err)
		os.Exit(1)
	}
	defer mongoRepo.Close(ctx)

	// Initialize and start the API server
	server := api.NewServer(cfg, mongoRepo, logger)
	go func() {
		if err := server.Start(); err != nil {
			logger.Error("Server failed to start", "error", err)
			cancel()
		}
	}()

	logger.Info("Server started", "port", cfg.Server.Port)

	// Wait for shutdown signal
	<-shutdown
	logger.Info("Shutting down server...")

	// Create a timeout context for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exited properly")
}

func setupLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
