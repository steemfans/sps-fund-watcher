package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"github.com/ety001/sps-fund-watcher/internal/storage"
	"github.com/ety001/sps-fund-watcher/internal/telegram"
	protocolapi "github.com/steemit/steemutil/protocol/api"
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
func (bp *BlockProcessor) ProcessBlock(ctx context.Context, block *protocolapi.Block, blockNum int64) ([]*models.Operation, error) {
	// Parse block timestamp
	var blockTime time.Time
	if block.Timestamp != nil && block.Timestamp.Time != nil {
		blockTime = *block.Timestamp.Time
	} else {
		blockTime = time.Now()
	}

	var operations []*models.Operation

	for _, tx := range block.Transactions {
		for opIndex, protocolOp := range tx.Operations {
			// Get operation type and data from protocol.Operation interface
			opType := string(protocolOp.Type())

			// Convert operation data to map[string]interface{}
			opDataRaw := protocolOp.Data()
			var opData map[string]interface{}

			// Try to convert to map
			if dataMap, ok := opDataRaw.(map[string]interface{}); ok {
				opData = dataMap
			} else {
				// If it's a typed operation, marshal and unmarshal to get map
				dataJSON, err := json.Marshal(opDataRaw)
				if err != nil {
					continue
				}
				if err := json.Unmarshal(dataJSON, &opData); err != nil {
					continue
				}
			}

			// Extract accounts from operation data
			accounts := bp.extractAccounts(opType, opData)
			if len(accounts) == 0 {
				continue
			}

			// Create operation for each tracked account
			for _, account := range accounts {
				// Check if account is tracked
				if !bp.accounts[account] {
					continue
				}

				// Create operation model
				op := &models.Operation{
					BlockNum:  blockNum,
					TrxID:     tx.TransactionId,
					OpInTrx:   opIndex,
					Account:   account,
					OpType:    opType,
					OpData:    opData,
					Timestamp: blockTime,
				}

				operations = append(operations, op)
			}
		}
	}

	return operations, nil
}

// extractAccounts extracts account names from operation data
// Returns a slice of accounts involved in the operation
func (bp *BlockProcessor) extractAccounts(opType string, opData map[string]interface{}) []string {
	var accounts []string

	// Try common account fields
	if account, ok := opData["account"].(string); ok {
		accounts = append(accounts, account)
	}
	if account, ok := opData["owner"].(string); ok {
		accounts = append(accounts, account)
	}

	// For transfer operations, include both from and to
	if opType == "transfer" {
		if from, ok := opData["from"].(string); ok {
			accounts = append(accounts, from)
		}
		if to, ok := opData["to"].(string); ok {
			accounts = append(accounts, to)
		}
	} else {
		// For other operations, try from field
		if account, ok := opData["from"].(string); ok {
			accounts = append(accounts, account)
		}
	}

	// Remove duplicates
	accountMap := make(map[string]bool)
	var uniqueAccounts []string
	for _, acc := range accounts {
		if !accountMap[acc] {
			accountMap[acc] = true
			uniqueAccounts = append(uniqueAccounts, acc)
		}
	}

	return uniqueAccounts
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
