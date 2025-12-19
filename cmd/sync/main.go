package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"github.com/ety001/sps-fund-watcher/internal/sync"
	"gopkg.in/yaml.v3"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	flag.Parse()

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

