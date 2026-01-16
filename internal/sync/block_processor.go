package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"github.com/ety001/sps-fund-watcher/internal/storage"
	"github.com/ety001/sps-fund-watcher/internal/telegram"
	"github.com/steemit/steemutil/protocol"
	protocolapi "github.com/steemit/steemutil/protocol/api"
)

// TelegramNotificationRule holds a notification rule configuration
type TelegramNotificationRule struct {
	Config         models.TelegramUserConfig
	NotifyOps      map[string]bool
	NotifyAllOps   bool
	NotifyAccounts map[string]bool
	NotifyAllAccts bool
}

// BlockProcessor processes blocks and extracts operations
type BlockProcessor struct {
	storage           *storage.MongoDB
	telegramClient    *telegram.Client
	notificationRules []TelegramNotificationRule
	accounts          map[string]bool
	globalTemplate    string
}

// NewBlockProcessor creates a new block processor
func NewBlockProcessor(
	storage *storage.MongoDB,
	telegramClient *telegram.Client,
	userConfigs []models.TelegramUserConfig,
	accounts []string,
	globalMessageTemplate string,
) *BlockProcessor {
	// Create account map for fast lookup
	accountMap := make(map[string]bool)
	for _, account := range accounts {
		accountMap[account] = true
	}

	// Prepare notification rules
	var rules []TelegramNotificationRule
	for _, userConfig := range userConfigs {
		// Create notify operations map
		notifyOpsMap := make(map[string]bool)
		notifyAllOps := len(userConfig.NotifyOperations) == 0
		if !notifyAllOps {
			for _, opType := range userConfig.NotifyOperations {
				notifyOpsMap[opType] = true
			}
		}

		// Create notify accounts map
		notifyAcctsMap := make(map[string]bool)
		notifyAllAccts := len(userConfig.Accounts) == 0
		if !notifyAllAccts {
			for _, account := range userConfig.Accounts {
				notifyAcctsMap[account] = true
			}
		}

		rules = append(rules, TelegramNotificationRule{
			Config:         userConfig,
			NotifyOps:      notifyOpsMap,
			NotifyAllOps:   notifyAllOps,
			NotifyAccounts: notifyAcctsMap,
			NotifyAllAccts: notifyAllAccts,
		})
	}

	return &BlockProcessor{
		storage:           storage,
		telegramClient:    telegramClient,
		notificationRules: rules,
		accounts:          accountMap,
		globalTemplate:    globalMessageTemplate,
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

// ProcessOperations processes operations from OperationObject array
// and extracts operations for tracked accounts. This handles both regular and virtual operations.
func (bp *BlockProcessor) ProcessOperations(ctx context.Context, ops []*protocol.OperationObject) ([]*models.Operation, error) {
	var operations []*models.Operation

	for _, opObj := range ops {
		// Parse timestamp from OperationObject
		var opTime time.Time
		if opObj.Timestamp != nil && opObj.Timestamp.Time != nil {
			opTime = *opObj.Timestamp.Time
		} else {
			opTime = time.Now()
		}

		// Get operation type and data
		opType := string(opObj.Operation.Type())

		// Convert operation data to map[string]interface{}
		opDataRaw := opObj.Operation.Data()
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

			// For virtual operations, TransactionID is usually empty
			// Use a combination of block number and virtual op number as unique identifier
			trxID := opObj.TransactionID
			if trxID == "" {
				if opObj.VirtualOperation > 0 {
					// Virtual operation: use virtual op number as identifier
					trxID = fmt.Sprintf("virtual_%d_%d", opObj.BlockNumber, opObj.VirtualOperation)
				} else {
					// Regular operation with empty TransactionID (should not happen, but handle gracefully)
					// Use transaction_in_block and op_in_trx as fallback identifier
					trxID = fmt.Sprintf("regular_%d_%d_%d", opObj.BlockNumber, opObj.TransactionInBlock, opObj.OperationInTransaction)
				}
			}

			// Create operation model
			op := &models.Operation{
				BlockNum:  int64(opObj.BlockNumber),
				TrxID:     trxID,
				OpInTrx:   int(opObj.OperationInTransaction),
				Account:   account,
				OpType:    opType,
				OpData:    opData,
				Timestamp: opTime,
			}

			operations = append(operations, op)
		}
	}

	return operations, nil
}

// ProcessVirtualOperations is kept for backward compatibility
// It now delegates to ProcessOperations
func (bp *BlockProcessor) ProcessVirtualOperations(ctx context.Context, ops []*protocol.OperationObject, blockNum int64) ([]*models.Operation, error) {
	return bp.ProcessOperations(ctx, ops)
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

	case "proposal_pay":
		if receiver := extractString("receiver"); receiver != "" {
			accounts = append(accounts, receiver)
		}

	case "author_reward":
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "curation_reward":
		if curator := extractString("curator"); curator != "" {
			accounts = append(accounts, curator)
		}
		if commentAuthor := extractString("comment_author"); commentAuthor != "" {
			accounts = append(accounts, commentAuthor)
		}

	case "shutdown_witness":
		if owner := extractString("owner"); owner != "" {
			accounts = append(accounts, owner)
		}

	case "comment_payout_update":
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "return_vesting_delegation":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
		}

	case "comment_benefactor_reward":
		if benefactor := extractString("benefactor"); benefactor != "" {
			accounts = append(accounts, benefactor)
		}
		if author := extractString("author"); author != "" {
			accounts = append(accounts, author)
		}

	case "producer_reward":
		if producer := extractString("producer"); producer != "" {
			accounts = append(accounts, producer)
		}

	case "hardfork23":
		if account := extractString("account"); account != "" {
			accounts = append(accounts, account)
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

// shouldNotifyForRule checks if an operation should be notified for a specific rule
func (bp *BlockProcessor) shouldNotifyForRule(rule TelegramNotificationRule, op *models.Operation) bool {
	// Check if operation type matches
	opTypeMatches := rule.NotifyAllOps
	if !opTypeMatches {
		opTypeMatches = rule.NotifyOps[op.OpType]
	}
	if !opTypeMatches {
		return false
	}

	// Check if account matches
	accountMatches := rule.NotifyAllAccts
	if !accountMatches {
		accountMatches = rule.NotifyAccounts[op.Account]
	}
	if !accountMatches {
		return false
	}

	// Check operation-level filters
	if !bp.passesOperationFilters(rule.Config.OperationFilters, op) {
		return false
	}

	return true
}

// passesOperationFilters checks if an operation passes all configured filters
func (bp *BlockProcessor) passesOperationFilters(filters map[string]models.OperationFilter, op *models.Operation) bool {
	if filters == nil {
		return true
	}

	// Check if there's a filter for this operation type
	filter, exists := filters[op.OpType]
	if !exists {
		return true
	}

	// Apply different filter logic based on opType
	switch op.OpType {
	case "transfer":
		return bp.passesTransferFilter(filter, op.OpData)
	default:
		return true
	}
}

// passesTransferFilter checks if a transfer operation passes the filter
func (bp *BlockProcessor) passesTransferFilter(filter models.OperationFilter, opData map[string]interface{}) bool {
	// If no whitelist configured, pass all checks
	if len(filter.IgnoreToAddresses) == 0 {
		return true
	}

	// Check if recipient is in whitelist
	toAddr, ok := opData["to"].(string)
	if !ok {
		return true
	}

	// If recipient is in whitelist, return false (don't notify)
	for _, ignoreAddr := range filter.IgnoreToAddresses {
		if toAddr == ignoreAddr {
			return false
		}
	}

	return true
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

	// Send Telegram notifications for each configured rule
	if bp.telegramClient != nil {
		for _, rule := range bp.notificationRules {
			for _, op := range operations {
				// Check if should notify for this rule
				if !bp.shouldNotifyForRule(rule, op) {
					continue
				}

				// Format message
				var message string
				if rule.Config.MessageTemplate != "" {
					// Use rule-specific template
					message = telegram.FormatOperationMessageWithTemplate(
						rule.Config.MessageTemplate,
						op.Account,
						op.OpType,
						op.OpData,
						op.BlockNum,
						op.Timestamp,
					)
				} else if bp.globalTemplate != "" {
					// Use global template
					message = telegram.FormatOperationMessageWithTemplate(
						bp.globalTemplate,
						op.Account,
						op.OpType,
						op.OpData,
						op.BlockNum,
						op.Timestamp,
					)
				} else {
					// Use default format
					message = telegram.FormatOperationMessage(
						op.Account,
						op.OpType,
						op.OpData,
						op.BlockNum,
						op.Timestamp,
					)
				}

				if err := bp.telegramClient.SendMessage(message); err != nil {
					fmt.Printf("Failed to send Telegram notification for rule %s: %v\n",
						rule.Config.Name, err)
				}
			}
		}
	}

	return nil
}
