package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"github.com/ety001/sps-fund-watcher/internal/telegram"
	"gopkg.in/yaml.v3"
)

func main() {
	configPath := flag.String("config", "configs/config.temp.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check if Telegram is enabled
	if !config.Telegram.Enabled {
		log.Fatalf("Telegram is not enabled in configuration")
	}

	// Check if bot token and channel ID are set
	if config.Telegram.BotToken == "" {
		log.Fatalf("Telegram bot_token is not set in configuration")
	}
	if config.Telegram.ChannelID == "" {
		log.Fatalf("Telegram channel_id is not set in configuration")
	}

	// Create Telegram client
	client := telegram.NewClient(config.Telegram.BotToken, config.Telegram.ChannelID)

	// Prepare test operation data
	testOpData := map[string]interface{}{
		"from":   "test-account",
		"to":     "test-recipient",
		"amount": "100.000 STEEM",
		"memo":   "Test message",
	}

	// Format message
	var message string
	if config.Telegram.MessageTemplate != "" {
		// Use custom template
		message = telegram.FormatOperationMessageWithTemplate(
			config.Telegram.MessageTemplate,
			"test-account",
			"transfer",
			testOpData,
			123456789,
			time.Now(),
		)
		log.Println("Using custom message template")
	} else {
		// Use default template
		message = telegram.FormatOperationMessage(
			"test-account",
			"transfer",
			testOpData,
			123456789,
			time.Now(),
		)
		log.Println("Using default message template")
	}

	// Print message preview
	fmt.Println("\n=== Message Preview ===")
	fmt.Println(message)
	fmt.Println("======================")

	// Send message
	log.Printf("Sending test message to Telegram channel %s...", config.Telegram.ChannelID)
	if err := client.SendMessage(message); err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	log.Println("✅ Test message sent successfully!")
}

func loadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

