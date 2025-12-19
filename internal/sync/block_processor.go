package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"github.com/ety001/sps-fund-watcher/internal/storage"
	"github.com/ety001/sps-fund-watcher/internal/telegram"
)

// BlockProcessor processes blocks and extracts operations
type BlockProcessor struct {
	storage      *storage.MongoDB
	telegram     *telegram.Client
	accounts     map[string]bool
	notifyOps    map[string]bool
	notifyAllOps bool
}

// NewBlockProcessor creates a new block processor
func NewBlockProcessor(
	storage *storage.MongoDB,
	telegram *telegram.Client,
	accounts []string,
	notifyOperations []string,
) *BlockProcessor {
	// Create account map for fast lookup
	accountMap := make(map[string]bool)
	for _, account := range accounts {
		accountMap[account] = true
	}

	// Create notify operations map
	notifyOpsMap := make(map[string]bool)
	notifyAllOps := len(notifyOperations) == 0
	if !notifyAllOps {
		for _, opType := range notifyOperations {
			notifyOpsMap[opType] = true
		}
	}

	return &BlockProcessor{
		storage:      storage,
		telegram:     telegram,
		accounts:     accountMap,
		notifyOps:    notifyOpsMap,
		notifyAllOps: notifyAllOps,
	}
}

// ProcessBlock processes a block and extracts operations for tracked accounts
func (bp *BlockProcessor) ProcessBlock(ctx context.Context, block *Block, blockNum int64) ([]*models.Operation, error) {
	blockTime, err := time.Parse("2006-01-02T15:04:05", block.Timestamp)
	if err != nil {
		// Try alternative format
		blockTime, err = time.Parse(time.RFC3339, block.Timestamp)
		if err != nil {
			blockTime = time.Now()
		}
	}

	var operations []*models.Operation

	for _, tx := range block.Transactions {
		for _, rawOp := range tx.Operations {
			// Operations are in format: [type, data]
			opArray, ok := rawOp.([]interface{})
			if !ok || len(opArray) < 2 {
				continue
			}

			opType, ok := opArray[0].(string)
			if !ok {
				continue
			}

			opData, ok := opArray[1].(map[string]interface{})
			if !ok {
				continue
			}

			// Extract account from operation data
			account := bp.extractAccount(opType, opData)
			if account == "" {
				continue
			}

			// Check if account is tracked
			if !bp.accounts[account] {
				continue
			}

			// Create operation model
			op := &models.Operation{
				BlockNum:  blockNum,
				TrxID:     tx.TransactionID,
				Account:   account,
				OpType:    opType,
				OpData:    opData,
				Timestamp: blockTime,
			}

			operations = append(operations, op)
		}
	}

	return operations, nil
}

// extractAccount extracts the account name from operation data
func (bp *BlockProcessor) extractAccount(opType string, opData map[string]interface{}) string {
	// Try common account fields
	if account, ok := opData["account"].(string); ok {
		return account
	}
	if account, ok := opData["from"].(string); ok {
		return account
	}
	if account, ok := opData["owner"].(string); ok {
		return account
	}

	// For transfer operations, check both from and to
	if opType == "transfer" {
		if from, ok := opData["from"].(string); ok {
			return from
		}
		if to, ok := opData["to"].(string); ok {
			return to
		}
	}

	return ""
}

// SaveOperations saves operations to storage and sends notifications
func (bp *BlockProcessor) SaveOperations(ctx context.Context, operations []*models.Operation) error {
	if len(operations) == 0 {
		return nil
	}

	// Save all operations to MongoDB
	if err := bp.storage.InsertOperations(ctx, operations); err != nil {
		return fmt.Errorf("failed to insert operations: %w", err)
	}

	// Send Telegram notifications for matching operations
	if bp.telegram != nil {
		for _, op := range operations {
			shouldNotify := bp.notifyAllOps
			if !shouldNotify {
				shouldNotify = bp.notifyOps[op.OpType]
			}

			if shouldNotify {
				message := telegram.FormatOperationMessage(
					op.Account,
					op.OpType,
					op.OpData,
					op.BlockNum,
					op.Timestamp,
				)

				if err := bp.telegram.SendMessage(message); err != nil {
					// Log error but don't fail the sync
					fmt.Printf("Failed to send Telegram notification: %v\n", err)
				}
			}
		}
	}

	return nil
}

