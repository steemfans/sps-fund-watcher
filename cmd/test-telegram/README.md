# Telegram Test Tool

This tool allows you to test the Telegram notification functionality by sending a test message using the configuration from your config file.

## Usage

```bash
# Use default config file (configs/config.temp.yaml)
go run cmd/test-telegram/main.go

# Use custom config file
go run cmd/test-telegram/main.go -config configs/config.yaml
```

## Features

- Reads Telegram configuration from YAML config file
- Supports custom message templates
- Shows message preview before sending
- Validates configuration before sending

## Configuration

The tool uses the following configuration from your config file:

- `telegram.enabled` - Must be `true`
- `telegram.bot_token` - Telegram bot token
- `telegram.channel_id` - Telegram channel ID
- `telegram.message_template` - Optional custom message template

## Example Output

```
Using custom message template

=== Message Preview ===
<b>🔔 New Operation</b>

<b>Account:</b> <code>test-account</code>
<b>Type:</b> <code>transfer</code>
<b>Block:</b> <code>123456789</code>
<b>Time:</b> <code>2025-01-15 10:30:45 UTC</code>

<b>Details:</b>
  • <b>from:</b> <code>test-account</code>
  • <b>to:</b> <code>test-recipient</code>
  • <b>amount:</b> <code>100.000 STEEM</code>
======================

Sending test message to Telegram channel -1003526962609...
✅ Test message sent successfully!
```
