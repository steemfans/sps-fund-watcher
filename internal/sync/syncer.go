package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"github.com/ety001/sps-fund-watcher/internal/storage"
	"github.com/ety001/sps-fund-watcher/internal/telegram"
	"github.com/steemit/steemgosdk"
)

// Syncer handles the synchronization process
type Syncer struct {
	steemAPI  *steemgosdk.API
	storage   *storage.MongoDB
	telegram  *telegram.Client
	processor *BlockProcessor
	config    *models.Config
	stopChan  chan struct{}
}

// NewSyncer creates a new syncer
func NewSyncer(config *models.Config) (*Syncer, error) {
	// Initialize Steem client using steemgosdk
	client := steemgosdk.GetClient(config.Steem.APIURL)
	steemAPI := client.GetAPI()

	// Initialize MongoDB storage
	mongoStorage, err := storage.NewMongoDB(config.MongoDB.URI, config.MongoDB.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MongoDB: %w", err)
	}

	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := mongoStorage.CreateIndexes(ctx); err != nil {
		log.Printf("Warning: failed to create indexes: %v", err)
	}

	// Initialize Telegram client if enabled
	var tgClient *telegram.Client
	if config.Telegram.Enabled && config.Telegram.BotToken != "" && config.Telegram.ChannelID != "" {
		tgClient = telegram.NewClient(config.Telegram.BotToken, config.Telegram.ChannelID)
	}

	// Initialize block processor
	processor := NewBlockProcessor(
		mongoStorage,
		tgClient,
		config.Steem.Accounts,
		config.Telegram.NotifyOperations,
	)

	return &Syncer{
		steemAPI:  steemAPI,
		storage:   mongoStorage,
		telegram:  tgClient,
		processor: processor,
		config:    config,
		stopChan:  make(chan struct{}),
	}, nil
}

// Start starts the synchronization process
func (s *Syncer) Start(ctx context.Context) error {
	log.Println("Starting sync service...")

	// Get current sync state
	syncState, err := s.storage.GetSyncState(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sync state: %w", err)
	}

	// Determine start block
	startBlock := s.config.Steem.StartBlock
	if syncState.LastBlock > 0 && syncState.LastBlock >= startBlock {
		startBlock = syncState.LastBlock + 1
		log.Printf("Resuming from block %d", startBlock)
	} else {
		log.Printf("Starting from configured block %d", startBlock)
	}

	// Sync loop
	ticker := time.NewTicker(3 * time.Second) // Check every 3 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Sync service stopped by context")
			return ctx.Err()
		case <-s.stopChan:
			log.Println("Sync service stopped")
			return nil
		case <-ticker.C:
			if err := s.syncBlocks(ctx, startBlock); err != nil {
				log.Printf("Error syncing blocks: %v", err)
				// Continue syncing despite errors
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// syncBlocks syncs blocks from startBlock to latest irreversible block
func (s *Syncer) syncBlocks(ctx context.Context, startBlock int64) error {
	// Get latest irreversible block
	dgp, err := s.steemAPI.GetDynamicGlobalProperties()
	if err != nil {
		return fmt.Errorf("failed to get dynamic global properties: %w", err)
	}
	latestIrreversible := int64(dgp.LastIrreversibleBlockNum)

	if startBlock > latestIrreversible {
		// No new blocks to sync
		return nil
	}

	// Sync blocks in batches
	batchSize := int64(10)
	currentBlock := startBlock
	lastSyncedBlock := startBlock - 1

	for currentBlock <= latestIrreversible {
		// Process batch
		endBlock := currentBlock + batchSize - 1
		if endBlock > latestIrreversible {
			endBlock = latestIrreversible
		}

		for blockNum := currentBlock; blockNum <= endBlock; blockNum++ {
			block, err := s.steemAPI.GetBlock(uint(blockNum))
			if err != nil {
				return fmt.Errorf("failed to get block %d: %w", blockNum, err)
			}

			// Process block
			operations, err := s.processor.ProcessBlock(ctx, block, blockNum)
			if err != nil {
				return fmt.Errorf("failed to process block %d: %w", blockNum, err)
			}

			// Save operations
			if len(operations) > 0 {
				if err := s.processor.SaveOperations(ctx, operations); err != nil {
					return fmt.Errorf("failed to save operations for block %d: %w", blockNum, err)
				}
				log.Printf("Block %d: saved %d operations", blockNum, len(operations))
			}

			lastSyncedBlock = blockNum
		}

		// Update sync state
		if err := s.storage.UpdateSyncState(ctx, lastSyncedBlock, latestIrreversible); err != nil {
			log.Printf("Warning: failed to update sync state: %v", err)
		}

		currentBlock = endBlock + 1

		// Small delay to avoid overwhelming the API
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Synced blocks %d to %d", startBlock, lastSyncedBlock)
	return nil
}

// Stop stops the syncer
func (s *Syncer) Stop() {
	close(s.stopChan)
}

// Close closes all connections
func (s *Syncer) Close() error {
	return s.storage.Close()
}
