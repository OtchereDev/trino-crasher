package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OtchereDev/trino-crasher/internal/repo"
	"github.com/OtchereDev/trino-crasher/internal/view"
	"github.com/OtchereDev/trino-crasher/pkg/config"
)

// SimpleLogger implements the logger.Logger interface for demonstration
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string) {
	log.Printf("[DEBUG] %s", msg)
}

func (l *SimpleLogger) Info(msg string) {
	log.Printf("[INFO] %s", msg)
}

func (l *SimpleLogger) Warn(msg string) {
	log.Printf("[WARN] %s", msg)
}

func (l *SimpleLogger) Error(msg string) {
	log.Printf("[ERROR] %s", msg)
}

func main() {
	// Create logger
	logger := &SimpleLogger{}

	// Create configuration
	cfg := config.DefaultConfig()

	// Override with environment variables if provided
	if dsn := os.Getenv("KETO_DSN"); dsn != "" {
		cfg.KetoDSN = dsn
	}
	if dsn := os.Getenv("AUTH_DB_DSN"); dsn != "" {
		cfg.AuthDBDSN = dsn
	}
	if dsn := os.Getenv("ASSET_DB_DSN"); dsn != "" {
		cfg.AssetDBDSN = dsn
	}
	if dsn := os.Getenv("MONGO_DSN"); dsn != "" {
		cfg.MongoDSN = dsn
	}
	if db := os.Getenv("MONGO_DATABASE"); db != "" {
		cfg.MongoDatabase = db
	}

	// Parse refresh intervals if provided
	if interval := os.Getenv("IDENTITY_REFRESH_INTERVAL"); interval != "" {
		if duration, err := time.ParseDuration(interval); err == nil {
			cfg.IdentityRefreshInterval = duration
		}
	}
	if interval := os.Getenv("DEVICE_REFRESH_INTERVAL"); interval != "" {
		if duration, err := time.ParseDuration(interval); err == nil {
			cfg.DeviceRefreshInterval = duration
		}
	}

	log.Println("Starting Trino-Crasher with in-memory stores...")
	log.Printf("Identity Refresh Interval: %v", cfg.IdentityRefreshInterval)
	log.Printf("Device Refresh Interval: %v", cfg.DeviceRefreshInterval)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create MaterializedViewManager
	viewManager, err := view.NewMaterializedViewManager(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to create view manager: %v", err)
	}
	defer viewManager.Close()

	// Initialize stores
	log.Println("Initializing in-memory stores...")
	if err := viewManager.InitializeTables(ctx); err != nil {
		log.Fatalf("Failed to initialize stores: %v", err)
	}
	log.Println("✓ Stores initialized successfully")

	// Print store statistics
	log.Printf("Identity store size: %d records", viewManager.GetActiveIdentityStoreSize())
	log.Printf("Device store size: %d records", viewManager.GetActiveDeviceStoreSize())

	// Start periodic refresh
	log.Println("Starting periodic refresh...")
	viewManager.StartPeriodicRefresh(ctx)

	// Create repository
	repository := repo.NewRepo(viewManager, logger)

	// Example: Search identities
	identityResults, err := repository.SearchIdentities(ctx, "user@example.com", "test", false)
	if err != nil {
		log.Printf("Failed to search identities: %v", err)
	} else {
		log.Printf("Identity search returned %d results", len(identityResults))
	}

	// Example: Search devices
	deviceResults, total, err := repository.SearchDevices(ctx, "user@example.com", "test", false, 1, 10)
	if err != nil {
		log.Printf("Failed to search devices: %v", err)
	} else {
		log.Printf("Device search returned %d results (total: %d)", len(deviceResults), total)
	}

	// Wait for interrupt signal
	log.Println("✓ Service is running. Press Ctrl+C to stop...")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	cancel()

	// Give some time for goroutines to stop
	time.Sleep(2 * time.Second)
	fmt.Println("✓ Shutdown complete")
}
