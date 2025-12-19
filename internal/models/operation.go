package models

import "time"

// Operation represents a Steem blockchain operation
type Operation struct {
	ID        string                 `bson:"_id,omitempty" json:"id"`
	BlockNum  int64                  `bson:"block_num" json:"block_num"`
	TrxID     string                 `bson:"trx_id" json:"trx_id"`
	Account   string                 `bson:"account" json:"account"`
	OpType    string                 `bson:"op_type" json:"op_type"`
	OpData    map[string]interface{} `bson:"op_data" json:"op_data"`
	Timestamp time.Time              `bson:"timestamp" json:"timestamp"`
	CreatedAt time.Time              `bson:"created_at" json:"created_at"`
}

// SyncState represents the current sync state
type SyncState struct {
	ID                      string    `bson:"_id,omitempty" json:"id"`
	LastBlock               int64     `bson:"last_block" json:"last_block"`
	LastIrreversibleBlock   int64     `bson:"last_irreversible_block" json:"last_irreversible_block"`
	UpdatedAt               time.Time `bson:"updated_at" json:"updated_at"`
}

// OperationResponse represents a paginated operation response
type OperationResponse struct {
	Operations []Operation `json:"operations"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	HasMore    bool        `json:"has_more"`
}

