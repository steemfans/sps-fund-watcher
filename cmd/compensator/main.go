package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"github.com/ety001/sps-fund-watcher/internal/storage"
	"github.com/ety001/sps-fund-watcher/internal/sync"
	"github.com/steemit/steemgosdk"
	"gopkg.in/yaml.v3"
)

func main() {
	// Parse command line flags
	account := flag.String("account", "", "Account name to compensate")
	startBlock := flag.Int64("start", 0, "Start block number")
	endBlock := flag.Int64("end", 0, "End block number")
	flag.Parse()

	// Get config file path from remaining arguments
	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("Config file path is required")
	}
	configPath := args[0]

	// Validate inputs
	if *account == "" {
		log.Fatal("Account name is required (use -account flag)")
	}
	if *startBlock <= 0 {
		log.Fatal("Start block must be greater than 0 (use -start flag)")
	}
	if *endBlock <= 0 {
		log.Fatal("End block must be greater than 0 (use -end flag)")
	}
	if *startBlock > *endBlock {
		log.Fatalf("Start block (%d) must be less than or equal to end block (%d)", *startBlock, *endBlock)
	}

	log.Printf("Compensator started: account=%s, start=%d, end=%d, config=%s", *account, *startBlock, *endBlock, configPath)

	// Load configuration
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Steem API client
	client := steemgosdk.GetClient(config.Steem.APIURL)
	steemAPI := client.GetAPI()
	log.Printf("Steem API initialized: %s", config.Steem.APIURL)

	// Initialize MongoDB storage
	mongoStorage, err := storage.NewMongoDB(config.MongoDB.URI, config.MongoDB.Database)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}
	defer mongoStorage.Close()
	log.Printf("MongoDB initialized: %s/%s", config.MongoDB.URI, config.MongoDB.Database)

	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := mongoStorage.CreateIndexes(ctx); err != nil {
		log.Printf("Warning: failed to create indexes: %v", err)
	}

	// Initialize block processor with only the target account
	// Pass nil for Telegram client since we don't want notifications for historical data
	processor := sync.NewBlockProcessor(
		mongoStorage,
		nil,                // No Telegram notifications
		[]string{*account}, // Only track the specified account
		nil,                // No notify operations filter
		nil,                // No notify accounts filter
		"",                 // No message template
	)

	// Process blocks
	batchSize := config.Steem.BatchSize
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
	}
	log.Printf("Using batch size: %d", batchSize)

	ctx = context.Background()
	totalBlocks := *endBlock - *startBlock + 1
	log.Printf("Processing %d blocks from %d to %d", totalBlocks, *startBlock, *endBlock)

	currentBlock := *startBlock
	totalOperations := 0
	processedBlocks := 0

	for currentBlock <= *endBlock {
		// Calculate batch end
		batchEnd := currentBlock + batchSize - 1
		if batchEnd > *endBlock {
			batchEnd = *endBlock
		}

		log.Printf("Fetching blocks %d to %d...", currentBlock, batchEnd)

		// Get blocks in batch (GetBlocks to parameter is exclusive, so we use batchEnd+1)
		wrapBlocks, err := steemAPI.GetBlocks(uint(currentBlock), uint(batchEnd+1))
		if err != nil {
			log.Fatalf("Failed to get blocks %d to %d: %v", currentBlock, batchEnd, err)
		}

		log.Printf("Processing %d blocks in batch...", len(wrapBlocks))

		// Process each block in the batch
		for _, wrapBlock := range wrapBlocks {
			blockNum := int64(wrapBlock.BlockNum)

			// Process block to extract operations for the target account
			operations, err := processor.ProcessBlock(ctx, wrapBlock.Block, blockNum)
			if err != nil {
				log.Fatalf("Failed to process block %d: %v", blockNum, err)
			}

			// Store operations (InsertOperations handles duplicates via upsert)
			if len(operations) > 0 {
				if err := mongoStorage.InsertOperations(ctx, operations); err != nil {
					log.Fatalf("Failed to insert operations for block %d: %v", blockNum, err)
				}
				totalOperations += len(operations)
				log.Printf("Block %d: saved %d operations", blockNum, len(operations))
			}

			processedBlocks++
			if processedBlocks%10 == 0 {
				log.Printf("Progress: %d/%d blocks processed, %d operations saved", processedBlocks, totalBlocks, totalOperations)
			}
		}

		currentBlock = batchEnd + 1

		// Small delay to avoid overwhelming the API
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Compensation completed: processed %d blocks, saved %d operations for account %s", processedBlocks, totalOperations, *account)
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
