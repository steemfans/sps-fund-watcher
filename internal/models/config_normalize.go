package models

// NormalizeTelegramConfig converts old format to new format for backward compatibility
// Returns the user configs and a boolean indicating if new format is used
func NormalizeTelegramConfig(config *TelegramConfig) ([]TelegramUserConfig, bool) {
	// If new format is used (users field exists and non-empty), return as-is
	if len(config.Users) > 0 {
		return config.Users, true
	}

	// If old format is used (users is empty), convert to single default rule
	// This maintains backward compatibility
	return []TelegramUserConfig{
		{
			Name:              "default",
			Accounts:          config.Accounts,
			NotifyOperations:  config.NotifyOperations,
			OperationFilters:  nil, // No filters in old format
			MessageTemplate:   "", // Use global template
		},
	}, false
}
