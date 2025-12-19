package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SteemClient represents a client for Steem JSON-RPC API
type SteemClient struct {
	apiURL    string
	httpClient *http.Client
}

// NewSteemClient creates a new Steem API client
func NewSteemClient(apiURL string) *SteemClient {
	return &SteemClient{
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// JSONRPCRequest represents a JSON-RPC request
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Block represents a Steem block
type Block struct {
	Previous              string        `json:"previous"`
	Timestamp             string        `json:"timestamp"`
	Witness               string        `json:"witness"`
	TransactionMerkleRoot string        `json:"transaction_merkle_root"`
	Extensions            []interface{} `json:"extensions"`
	WitnessSignature      string        `json:"witness_signature"`
	Transactions          []Transaction `json:"transactions"`
	BlockID               string        `json:"block_id"`
	SigningKey            string        `json:"signing_key"`
	TransactionIds        []string      `json:"transaction_ids"`
}

// Transaction represents a Steem transaction
type Transaction struct {
	RefBlockNum    int64         `json:"ref_block_num"`
	RefBlockPrefix int64         `json:"ref_block_prefix"`
	Expiration     string        `json:"expiration"`
	Operations     []interface{} `json:"operations"`
	Extensions     []interface{} `json:"extensions"`
	Signatures     []string      `json:"signatures"`
	TransactionID  string        `json:"transaction_id"`
	BlockNum       int64         `json:"block_num"`
	TransactionNum int           `json:"transaction_num"`
}

// Operation represents a Steem operation (array format: [type, data])
type Operation struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// call makes a JSON-RPC call to the Steem API
func (c *SteemClient) call(method string, params []interface{}) (json.RawMessage, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var jsonResp JSONRPCResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if jsonResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: %s (code: %d)", jsonResp.Error.Message, jsonResp.Error.Code)
	}

	return jsonResp.Result, nil
}

// GetBlock retrieves a block by block number
func (c *SteemClient) GetBlock(blockNum int64) (*Block, error) {
	result, err := c.call("condenser_api.get_block", []interface{}{blockNum})
	if err != nil {
		return nil, err
	}

	var block Block
	if err := json.Unmarshal(result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

// GetDynamicGlobalProperties retrieves dynamic global properties
func (c *SteemClient) GetDynamicGlobalProperties() (map[string]interface{}, error) {
	result, err := c.call("condenser_api.get_dynamic_global_properties", []interface{}{})
	if err != nil {
		return nil, err
	}

	var props map[string]interface{}
	if err := json.Unmarshal(result, &props); err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
	}

	return props, nil
}

// GetLatestIrreversibleBlockNum returns the latest irreversible block number
func (c *SteemClient) GetLatestIrreversibleBlockNum() (int64, error) {
	props, err := c.GetDynamicGlobalProperties()
	if err != nil {
		return 0, err
	}

	lastIrreversibleBlockNum, ok := props["last_irreversible_block_num"]
	if !ok {
		return 0, fmt.Errorf("last_irreversible_block_num not found in properties")
	}

	// Handle different number types
	switch v := lastIrreversibleBlockNum.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected type for last_irreversible_block_num: %T", v)
	}
}

