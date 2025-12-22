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
	Enabled          bool     `yaml:"enabled"`
	BotToken         string   `yaml:"bot_token"`
	ChannelID        string   `yaml:"channel_id"`
	NotifyOperations []string `yaml:"notify_operations"` // Empty means notify all operations
}

// APIConfig contains API server configuration
type APIConfig struct {
	Port string `yaml:"port"`
	Host string `yaml:"host"`
}
