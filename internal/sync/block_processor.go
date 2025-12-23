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
	storage         *storage.MongoDB
	telegram        *telegram.Client
	accounts        map[string]bool
	notifyOps       map[string]bool
	notifyAllOps    bool
	notifyAccounts  map[string]bool
	notifyAllAccts  bool
	messageTemplate string
}

// NewBlockProcessor creates a new block processor
func NewBlockProcessor(
	storage *storage.MongoDB,
	telegram *telegram.Client,
	accounts []string,
	notifyOperations []string,
	notifyAccounts []string,
	messageTemplate string,
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

	// Create notify accounts map
	notifyAcctsMap := make(map[string]bool)
	notifyAllAccts := len(notifyAccounts) == 0
	if !notifyAllAccts {
		for _, account := range notifyAccounts {
			notifyAcctsMap[account] = true
		}
	}

	return &BlockProcessor{
		storage:         storage,
		telegram:        telegram,
		accounts:        accountMap,
		notifyOps:       notifyOpsMap,
		notifyAllOps:    notifyAllOps,
		notifyAccounts:  notifyAcctsMap,
		notifyAllAccts:  notifyAllAccts,
		messageTemplate: messageTemplate,
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
// Based on operation definitions in steemutil/protocol/operations.go
func (bp *BlockProcessor) extractAccounts(opType string, opData map[string]interface{}) []string {
	var accounts []string

	// Helper function to safely extract string field
	extractString := func(field string) string {
		if val, ok := opData[field].(string); ok && val != "" {
			return val
		}
		return ""
	}

	// Extract accounts based on operation type
	switch opType {
	case "vote":
		if voter := extractString("voter"); voter != "" {
			accounts = append(accounts, voter)
		}
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "comment":
		if parentAuthor := extractString("parent_author"); parentAuthor != "" {
			accounts = append(accounts, parentAuthor)
		}
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "transfer":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}

	case "transfer_to_vesting":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}

	case "withdraw_vesting":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}

	case "limit_order_create":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "limit_order_cancel":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "feed_publish":
		if publisher := extractString("publisher"); publisher != "" {
			accounts = append(accounts, publisher)
		}

	case "convert":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "account_create":
		if creator := extractString("creator"); creator != "" {
			accounts = append(accounts, creator)
		}
		if newAccountName := extractString("new_account_name"); newAccountName != "" {
			accounts = append(accounts, newAccountName)
		}

	case "account_update":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}

	case "witness_update":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "account_witness_vote":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}
		if witness := extractString("witness"); witness != "" {
			accounts = append(accounts, witness)
		}

	case "account_witness_proxy":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}
		if proxy := extractString("proxy"); proxy != "" {
			accounts = append(accounts, proxy)
		}

	case "delete_comment":
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "comment_options":
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "set_withdraw_vesting_route":
		if fromAccount := extractString("from_account"); fromAccount != "" {
			accounts = append(accounts, fromAccount)
		}
		if toAccount := extractString("to_account"); toAccount != "" {
			accounts = append(accounts, toAccount)
		}

	case "limit_order_create2":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "claim_account":
		if creator := extractString("creator"); creator != "" {
			accounts = append(accounts, creator)
		}

	case "create_claimed_account":
		if creator := extractString("creator"); creator != "" {
			accounts = append(accounts, creator)
		}
		if newAccountName := extractString("new_account_name"); newAccountName != "" {
			accounts = append(accounts, newAccountName)
		}

	case "request_account_recovery":
		if recoveryAccount := extractString("recovery_account"); recoveryAccount != "" {
			accounts = append(accounts, recoveryAccount)
		}
		if accountToRecover := extractString("account_to_recover"); accountToRecover != "" {
			accounts = append(accounts, accountToRecover)
		}

	case "recover_account":
		if accountToRecover := extractString("account_to_recover"); accountToRecover != "" {
			accounts = append(accounts, accountToRecover)
		}

	case "change_recovery_account":
		if accountToRecover := extractString("account_to_recover"); accountToRecover != "" {
			accounts = append(accounts, accountToRecover)
		}
		if newRecoveryAccount := extractString("new_recovery_account"); newRecoveryAccount != "" {
			accounts = append(accounts, newRecoveryAccount)
		}

	case "escrow_transfer":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}
		if agent := extractString("agent"); agent != "" {
			accounts = append(accounts, agent)
		}

	case "escrow_dispute":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}
		if agent := extractString("agent"); agent != "" {
			accounts = append(accounts, agent)
		}
		if who := extractString("who"); who != "" {
			accounts = append(accounts, who)
		}

	case "escrow_release":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}
		if agent := extractString("agent"); agent != "" {
			accounts = append(accounts, agent)
		}
		if who := extractString("who"); who != "" {
			accounts = append(accounts, who)
		}
		if receiver := extractString("receiver"); receiver != "" {
			accounts = append(accounts, receiver)
		}

	case "escrow_approve":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}
		if agent := extractString("agent"); agent != "" {
			accounts = append(accounts, agent)
		}
		if who := extractString("who"); who != "" {
			accounts = append(accounts, who)
		}

	case "transfer_to_savings":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}

	case "transfer_from_savings":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}

	case "cancel_transfer_from_savings":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}

	case "decline_voting_rights":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}

	case "reset_account":
		if resetAccount := extractString("reset_account"); resetAccount != "" {
			accounts = append(accounts, resetAccount)
		}
		if accountToReset := extractString("account_to_reset"); accountToReset != "" {
			accounts = append(accounts, accountToReset)
		}

	case "set_reset_account":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}
		if currentResetAccount := extractString("current_reset_account"); currentResetAccount != "" {
			accounts = append(accounts, currentResetAccount)
		}
		if resetAccount := extractString("reset_account"); resetAccount != "" {
			accounts = append(accounts, resetAccount)
		}

	case "claim_reward_balance":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}

	case "delegate_vesting_shares":
		if delegator := extractString("delegator"); delegator != "" {
			accounts = append(accounts, delegator)
		}
		if delegatee := extractString("delegatee"); delegatee != "" {
			accounts = append(accounts, delegatee)
		}

	case "account_create_with_delegation":
		if creator := extractString("creator"); creator != "" {
			accounts = append(accounts, creator)
		}
		if newAccountName := extractString("new_account_name"); newAccountName != "" {
			accounts = append(accounts, newAccountName)
		}

	case "witness_set_properties":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "account_update2":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}

	case "create_proposal":
		if creator := extractString("creator"); creator != "" {
			accounts = append(accounts, creator)
		}
		if receiver := extractString("receiver"); receiver != "" {
			accounts = append(accounts, receiver)
		}

	case "update_proposal_votes":
		if voter := extractString("voter"); voter != "" {
			accounts = append(accounts, voter)
		}

	case "remove_proposal":
		if proposalOwner := extractString("proposal_owner"); proposalOwner != "" {
			accounts = append(accounts, proposalOwner)
		}

	case "claim_reward_balance2":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}

	case "vote2":
		if voter := extractString("voter"); voter != "" {
			accounts = append(accounts, voter)
		}
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "fill_convert_request":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "comment_reward":
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "liquidity_reward":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "interest":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "fill_vesting_withdraw":
		if fromAccount := extractString("from_account"); fromAccount != "" {
			accounts = append(accounts, fromAccount)
		}
		if toAccount := extractString("to_account"); toAccount != "" {
			accounts = append(accounts, toAccount)
		}

	case "fill_order":
		if currentOwner := extractString("current_owner"); currentOwner != "" {
			accounts = append(accounts, currentOwner)
		}
		if openOwner := extractString("open_owner"); openOwner != "" {
			accounts = append(accounts, openOwner)
		}

	case "fill_transfer_from_savings":
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}

	default:
		// Fallback: try common account fields for unknown operation types
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}
		if from := extractString("from"); from != "" {
			accounts = append(accounts, from)
		}
		if to := extractString("to"); to != "" {
			accounts = append(accounts, to)
		}
	}

	// Remove duplicates
	accountMap := make(map[string]bool)
	var uniqueAccounts []string
	for _, acc := range accounts {
		if acc != "" && !accountMap[acc] {
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
	// Only notify if both account and operation type match the filters
	if bp.telegram != nil {
		for _, op := range operations {
			// Check if operation type matches
			opTypeMatches := bp.notifyAllOps
			if !opTypeMatches {
				opTypeMatches = bp.notifyOps[op.OpType]
			}

			// Check if account matches
			accountMatches := bp.notifyAllAccts
			if !accountMatches {
				accountMatches = bp.notifyAccounts[op.Account]
			}

			// Only notify if both conditions are met
			if opTypeMatches && accountMatches {
				var message string
				if bp.messageTemplate != "" {
					message = telegram.FormatOperationMessageWithTemplate(
						bp.messageTemplate,
						op.Account,
						op.OpType,
						op.OpData,
						op.BlockNum,
						op.Timestamp,
					)
				} else {
					message = telegram.FormatOperationMessage(
						op.Account,
						op.OpType,
						op.OpData,
						op.BlockNum,
						op.Timestamp,
					)
				}

				if err := bp.telegram.SendMessage(message); err != nil {
					// Log error but don't fail the sync
					fmt.Printf("Failed to send Telegram notification: %v\n", err)
				}
			}
		}
	}

	return nil
}
