package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client represents a Telegram bot client
type Client struct {
	botToken   string
	channelID  string
	httpClient *http.Client
	apiURL     string
}

// NewClient creates a new Telegram bot client
func NewClient(botToken, channelID string) *Client {
	return &Client{
		botToken:  botToken,
		channelID: channelID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		apiURL: "https://api.telegram.org",
	}
}

// SendMessageRequest represents a Telegram sendMessage request
type SendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// TelegramResponse represents a Telegram API response
type TelegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// SendMessage sends a message to the configured Telegram channel
func (c *Client) SendMessage(text string) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", c.apiURL, c.botToken)

	req := SendMessageRequest{
		ChatID:    c.channelID,
		Text:      text,
		ParseMode: "HTML",
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var tgResp TelegramResponse
	if err := json.Unmarshal(body, &tgResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !tgResp.OK {
		return fmt.Errorf("telegram API error: %s", tgResp.Description)
	}

	return nil
}

// FormatOperationMessage formats an operation as a Telegram message
func FormatOperationMessage(account, opType string, opData map[string]interface{}, blockNum int64, timestamp time.Time) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "<b>🔔 New Operation</b>\n\n")
	builder.WriteString(fmt.Sprintf("<b>Account:</b> <code>%s</code>\n", account))
	builder.WriteString(fmt.Sprintf("<b>Type:</b> <code>%s</code>\n", opType))
	builder.WriteString(fmt.Sprintf("<b>Block:</b> <code>%d</code>\n", blockNum))
	builder.WriteString(fmt.Sprintf("<b>Time:</b> <code>%s</code>\n\n", timestamp.Format("2006-01-02 15:04:05 UTC")))

	// Format operation-specific data
	builder.WriteString("<b>Details:</b>\n")
	for key, value := range opData {
		// Skip internal fields
		if key == "memo" || key == "json_metadata" {
			continue
		}
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > 100 {
			valueStr = valueStr[:100] + "..."
		}
		fmt.Fprintf(&builder, "  • <b>%s:</b> <code>%s</code>\n", key, escapeHTML(valueStr))
	}

	return builder.String()
}

// FormatOperationMessageWithTemplate formats an operation using a custom template
// Template variables:
//   - {{.Account}} - Account name
//   - {{.OpType}} - Operation type
//   - {{.BlockNum}} - Block number
//   - {{.Timestamp}} - Timestamp (formatted as "2006-01-02 15:04:05 UTC")
//   - {{.Details}} - Operation details (formatted as key: value pairs)
func FormatOperationMessageWithTemplate(template string, account, opType string, opData map[string]interface{}, blockNum int64, timestamp time.Time) string {
	// Format details
	var detailsBuilder strings.Builder
	for key, value := range opData {
		// Skip internal fields
		if key == "memo" || key == "json_metadata" {
			continue
		}
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > 100 {
			valueStr = valueStr[:100] + "..."
		}
		fmt.Fprintf(&detailsBuilder, "  • <b>%s:</b> <code>%s</code>\n", key, escapeHTML(valueStr))
	}
	details := detailsBuilder.String()
	if details == "" {
		details = "  (no details)"
	}

	// Replace template variables
	result := template
	result = strings.ReplaceAll(result, "{{.Account}}", account)
	result = strings.ReplaceAll(result, "{{.OpType}}", opType)
	result = strings.ReplaceAll(result, "{{.BlockNum}}", fmt.Sprintf("%d", blockNum))
	result = strings.ReplaceAll(result, "{{.Timestamp}}", timestamp.Format("2006-01-02 15:04:05 UTC"))
	result = strings.ReplaceAll(result, "{{.Details}}", details)

	return result
}

// escapeHTML escapes HTML special characters
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ShouldNotify checks if an operation type should be notified based on the filter list
func ShouldNotify(opType string, notifyOperations []string) bool {
	// If notifyOperations is empty or nil, notify all operations
	if len(notifyOperations) == 0 {
		return true
	}

	// Check if operation type is in the notify list
	for _, notifyOp := range notifyOperations {
		if strings.EqualFold(notifyOp, opType) {
			return true
		}
	}

	return false
}
