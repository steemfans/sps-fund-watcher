package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"github.com/ety001/sps-fund-watcher/internal/sync"
	"gopkg.in/yaml.v3"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	lockFile := flag.String("lockfile", "", "Path to lock file (default: /tmp/sps-fund-watcher-sync.lock)")
	flag.Parse()

	// Determine lock file path
	lockFilePath := *lockFile
	if lockFilePath == "" {
		lockFilePath = "/tmp/sps-fund-watcher-sync.lock"
	}

	// Acquire file lock to prevent multiple instances
	lockFileHandle, err := acquireLock(lockFilePath)
	if err != nil {
		log.Fatalf("Failed to acquire lock: %v. Another sync instance may be running.", err)
	}
	defer releaseLock(lockFileHandle, lockFilePath)
	log.Printf("Lock acquired: %s", lockFilePath)

	// Load configuration
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create syncer
	syncer, err := sync.NewSyncer(config)
	if err != nil {
		log.Fatalf("Failed to create syncer: %v", err)
	}
	defer syncer.Close()

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start syncer in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := syncer.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		syncer.Stop()
		cancel()
	case err := <-errChan:
		log.Fatalf("Syncer error: %v", err)
	}

	log.Println("Sync service stopped")
}

func loadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// acquireLock acquires an exclusive file lock to prevent multiple instances
func acquireLock(lockFilePath string) (*os.File, error) {
	// Create lock file directory if it doesn't exist
	lockDir := filepath.Dir(lockFilePath)
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Open or create lock file
	file, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to acquire lock (another instance may be running): %w", err)
	}

	// Write PID to lock file for debugging
	pid := os.Getpid()
	pidStr := fmt.Sprintf("%d\n", pid)
	if err := file.Truncate(0); err != nil {
		log.Printf("Warning: failed to truncate lock file: %v", err)
	}
	if _, err := file.WriteString(pidStr); err != nil {
		// Log warning but don't fail
		log.Printf("Warning: failed to write PID to lock file: %v", err)
	}
	if err := file.Sync(); err != nil {
		log.Printf("Warning: failed to sync lock file: %v", err)
	}

	return file, nil
}

// releaseLock releases the file lock
func releaseLock(file *os.File, lockFilePath string) {
	if file != nil {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
		// Optionally remove lock file (but not necessary, as it will be reused)
		os.Remove(lockFilePath)
		log.Printf("Lock released: %s", lockFilePath)
	}
}
