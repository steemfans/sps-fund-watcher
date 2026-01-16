package models

// Config represents the application configuration
type Config struct {
	Steem    SteemConfig    `yaml:"steem"`
	MongoDB  MongoDBConfig  `yaml:"mongodb"`
	Telegram TelegramConfig `yaml:"telegram"`
	API      APIConfig      `yaml:"api"`
}

// SteemConfig contains Steem blockchain configuration
type SteemConfig struct {
	APIURL     string   `yaml:"api_url"`
	StartBlock int64    `yaml:"start_block"`
	Accounts   []string `yaml:"accounts"`
	BatchSize  int64    `yaml:"batch_size"` // Number of blocks to fetch in each batch
}

// MongoDBConfig contains MongoDB connection configuration
type MongoDBConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

// TelegramConfig contains Telegram bot configuration
type TelegramConfig struct {
	// 全局配置
	Enabled          bool                      `yaml:"enabled"`
	BotToken         string                    `yaml:"bot_token"`
	ChannelID        string                    `yaml:"channel_id"`
	MessageTemplate  string                    `yaml:"message_template"` // Global fallback template

	// 旧格式字段（用于向后兼容，当 users 为空时使用）
	Accounts         []string                  `yaml:"accounts"`
	NotifyOperations []string                  `yaml:"notify_operations"`

	// 新格式：支持多规则配置
	Users            []TelegramUserConfig      `yaml:"users"`
}

// TelegramUserConfig represents a single notification rule configuration
type TelegramUserConfig struct {
	Name              string                      `yaml:"name"`              // Rule identifier for logging
	Accounts          []string                    `yaml:"accounts"`          // Empty means all tracked accounts
	NotifyOperations  []string                    `yaml:"notify_operations"` // Empty means all operations
	OperationFilters  map[string]OperationFilter `yaml:"operation_filters"` // Key: operation type
	MessageTemplate   string                      `yaml:"message_template"`  // Optional custom template (overrides global)
}

// OperationFilter defines filters for a specific operation type
type OperationFilter struct {
	// For transfer operation
	IgnoreToAddresses []string `yaml:"ignore_to_addresses"` // Whitelist: don't notify if transfer to these addresses
}

// APIConfig contains API server configuration
type APIConfig struct {
	Port string `yaml:"port"`
	Host string `yaml:"host"`
}
