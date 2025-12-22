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
	log.Println("[DEBUG] Starting sync service...")
	log.Printf("[DEBUG] Configuration: API URL=%s, StartBlock=%d, BatchSize=%d, Accounts=%v",
		s.config.Steem.APIURL, s.config.Steem.StartBlock, s.config.Steem.BatchSize, s.config.Steem.Accounts)

	// Get current sync state
	syncState, err := s.storage.GetSyncState(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sync state: %w", err)
	}
	log.Printf("[DEBUG] Current sync state from DB: LastBlock=%d, LastIrreversibleBlock=%d, UpdatedAt=%v",
		syncState.LastBlock, syncState.LastIrreversibleBlock, syncState.UpdatedAt)

	// Determine start block
	startBlock := s.config.Steem.StartBlock
	if syncState.LastBlock > 0 && syncState.LastBlock >= startBlock {
		startBlock = syncState.LastBlock + 1
		log.Printf("[DEBUG] Resuming from block %d (DB LastBlock=%d, Config StartBlock=%d)", startBlock, syncState.LastBlock, s.config.Steem.StartBlock)
	} else {
		log.Printf("[DEBUG] Starting from configured block %d (DB LastBlock=%d)", startBlock, syncState.LastBlock)
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
			// Get current sync state before each sync cycle to ensure we start from the correct block
			currentState, err := s.storage.GetSyncState(ctx)
			if err != nil {
				log.Printf("[DEBUG] Error getting sync state: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			log.Printf("[DEBUG] Sync cycle: Current DB state - LastBlock=%d, LastIrreversibleBlock=%d",
				currentState.LastBlock, currentState.LastIrreversibleBlock)

			// Determine the actual start block from database state
			actualStartBlock := s.config.Steem.StartBlock
			if currentState.LastBlock > 0 && currentState.LastBlock >= s.config.Steem.StartBlock {
				actualStartBlock = currentState.LastBlock + 1
			}
			log.Printf("[DEBUG] Sync cycle: Calculated startBlock=%d (Config StartBlock=%d, DB LastBlock=%d)",
				actualStartBlock, s.config.Steem.StartBlock, currentState.LastBlock)

			if err := s.syncBlocks(ctx, actualStartBlock); err != nil {
				log.Printf("[DEBUG] Error syncing blocks: %v", err)
				// Continue syncing despite errors
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// syncBlocks syncs blocks from startBlock to latest irreversible block
func (s *Syncer) syncBlocks(ctx context.Context, startBlock int64) error {
	log.Printf("[DEBUG] syncBlocks called with startBlock=%d", startBlock)

	// Get latest irreversible block
	dgp, err := s.steemAPI.GetDynamicGlobalProperties()
	if err != nil {
		return fmt.Errorf("failed to get dynamic global properties: %w", err)
	}
	latestIrreversible := int64(dgp.LastIrreversibleBlockNum)
	log.Printf("[DEBUG] Latest irreversible block: %d", latestIrreversible)

	if startBlock > latestIrreversible {
		// No new blocks to sync
		log.Printf("[DEBUG] No new blocks to sync (startBlock=%d > latestIrreversible=%d)", startBlock, latestIrreversible)
		return nil
	}

	// Sync blocks in batches
	batchSize := s.config.Steem.BatchSize
	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}
	log.Printf("[DEBUG] Using batchSize=%d", batchSize)
	currentBlock := startBlock
	lastSyncedBlock := startBlock - 1

	for currentBlock <= latestIrreversible {
		// Process batch
		endBlock := currentBlock + batchSize - 1
		if endBlock > latestIrreversible {
			endBlock = latestIrreversible
		}
		log.Printf("[DEBUG] Processing batch: blocks %d to %d (total %d blocks)", currentBlock, endBlock, endBlock-currentBlock+1)

		// Get blocks in batch using GetBlocks (to is exclusive, so we use endBlock+1)
		log.Printf("[DEBUG] Calling GetBlocks(%d, %d)", currentBlock, endBlock+1)
		wrapBlocks, err := s.steemAPI.GetBlocks(uint(currentBlock), uint(endBlock+1))
		if err != nil {
			return fmt.Errorf("failed to get blocks %d to %d: %w", currentBlock, endBlock, err)
		}
		log.Printf("[DEBUG] GetBlocks returned %d blocks", len(wrapBlocks))

		// Process each block in the batch
		for i, wrapBlock := range wrapBlocks {
			blockNum := int64(wrapBlock.BlockNum)
			log.Printf("[DEBUG] Processing block %d/%d in batch (blockNum=%d)", i+1, len(wrapBlocks), blockNum)

			// Check current state before processing to avoid processing blocks we've already synced
			currentState, err := s.storage.GetSyncState(ctx)
			if err != nil {
				log.Printf("[DEBUG] Warning: failed to get sync state before processing block %d: %v", blockNum, err)
			} else {
				if blockNum <= currentState.LastBlock {
					log.Printf("[DEBUG] Skipping block %d: already synced (current LastBlock=%d)", blockNum, currentState.LastBlock)
					lastSyncedBlock = blockNum
					continue
				}
			}

			// Process block
			operations, err := s.processor.ProcessBlock(ctx, wrapBlock.Block, blockNum)
			if err != nil {
				return fmt.Errorf("failed to process block %d: %w", blockNum, err)
			}
			log.Printf("[DEBUG] Block %d: extracted %d operations", blockNum, len(operations))

			lastSyncedBlock = blockNum

			// Save operations and update sync state
			// Uses atomic $max operator to ensure last_block only increases (no transactions needed)
			log.Printf("[DEBUG] Saving operations and updating sync state for block %d (lastSyncedBlock=%d, latestIrreversible=%d)",
				blockNum, lastSyncedBlock, latestIrreversible)
			if err := s.storage.SaveOperationsAndUpdateSyncState(ctx, operations, lastSyncedBlock, latestIrreversible); err != nil {
				return fmt.Errorf("failed to save operations and update sync state for block %d: %w", blockNum, err)
			}
			log.Printf("[DEBUG] Successfully saved operations and updated sync state for block %d", blockNum)

			if len(operations) > 0 {
				log.Printf("[INFO] Block %d: saved %d operations", blockNum, len(operations))
			} else {
				log.Printf("[DEBUG] Block %d: no operations to save", blockNum)
			}
		}

		currentBlock = endBlock + 1
		log.Printf("[DEBUG] Batch completed. Next currentBlock=%d", currentBlock)

		// Small delay to avoid overwhelming the API
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("[INFO] Synced blocks %d to %d", startBlock, lastSyncedBlock)
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
