# SPS Fund Watcher

A comprehensive system for monitoring Steem blockchain operations for tracked accounts, with real-time Telegram notifications and a modern web interface.

## Features

- **Blockchain Sync Service**: Continuously syncs Steem blockchain data from a configured block height to the latest irreversible block
- **Account Tracking**: Monitor specific Steem accounts for all operations
- **Compensator Tool**: Fetch historical operations for accounts added after sync has started
- **Telegram Notifications**: Receive formatted notifications for selected operation types with advanced filtering:
  - **Multi-Rule Configuration**: Configure multiple notification rules with different filters
  - **Transfer Whitelist**: Ignore transfers to specific addresses (e.g., exchanges, savings)
  - **Custom Templates**: Use global or rule-specific message templates
  - **Backward Compatible**: Legacy configuration format still supported
- **REST API**: Full-featured API for querying operation history
- **Web Interface**: Modern React-based UI with pagination and filtering
- **Docker Deployment**: Single container deployment with supervisord and nginx

## Architecture

The system consists of four main components:

1. **Sync Service** (`cmd/sync`): Syncs blockchain data and stores it in MongoDB
2. **Compensator Tool** (`cmd/compensator`): Fetches historical operations for specific accounts within a block range
3. **API Service** (`cmd/api`): Provides REST API endpoints for the web frontend
4. **Web Frontend** (`web/`): React application built with Vite, Tailwind CSS, and shadcn/ui

All services run in a single Docker container managed by supervisord.

## Configuration

Edit `configs/config.yaml` to configure:

### Basic Configuration

```yaml
steem:
  api_url: "https://api.steem.fans"  # Steem API endpoint
  start_block: 50000000              # Starting block height
  batch_size: 100                    # Number of blocks to fetch per batch
  accounts:
    - "burndao.burn"                 # Accounts to track

mongodb:
  uri: "mongodb://localhost:27017"   # MongoDB connection string
  database: "sps_fund_watcher"        # Database name

api:
  port: "8080"                        # API server port
  host: "0.0.0.0"                     # API server host
```

### Telegram Configuration (Legacy Format - Still Supported)

```yaml
telegram:
  enabled: true                       # Enable Telegram notifications
  bot_token: "your_bot_token"         # Telegram bot token
  channel_id: "your_channel_id"       # Telegram channel ID
  accounts:                           # Accounts to notify (empty = all tracked)
    - "burndao.burn"
  notify_operations:                  # Operation types to notify
    - "transfer"
    - "account_update"
    # Empty list means notify all operations
  message_template: |                 # Optional custom message template
    🔔 <b>New Operation</b>
    <b>Account:</b> <code>{{.Account}}</code>
    <b>Type:</b> <code>{{.OpType}}</code>
    <b>Details:</b>
    {{.Details}}
```

### Telegram Configuration (New Multi-Rule Format)

The new format supports multiple notification rules with advanced filtering:

```yaml
telegram:
  enabled: true
  bot_token: "your_bot_token"         # Global bot token (shared by all rules)
  channel_id: "your_channel_id"       # Global channel ID (shared by all rules)

  # Global message template (used when rule doesn't have its own)
  message_template: |
    🔔 <b>New Operation</b>

    <b>Account:</b> <code>{{.Account}}</code>
    <b>Type:</b> <code>{{.OpType}}</code>
    <b>Block:</b> <code>{{.BlockNum}}</code>
    <b>Time:</b> <code>{{.Timestamp}}</code>

    <b>Details:</b>
    {{.Details}}

  # Multiple notification rules
  users:
    # Rule 1: Monitor transfers with whitelist filtering
    - name: "main-account-monitor"
      accounts:
        - "burndao.burn"
      notify_operations:
        - "transfer"
        - "account_update"
      operation_filters:
        transfer:
          # Whitelist: don't notify when transferring to these addresses
          ignore_to_addresses:
            - "exchange.account"
            - "savings.account"
            - "bittrex"
            - "poloniex"

    # Rule 2: Monitor all operations for all accounts
    - name: "all-operations-monitor"
      accounts: []                    # Empty = all tracked accounts
      notify_operations: []            # Empty = all operation types
      operation_filters: {}            # No filters

    # Rule 3: Vote monitoring with custom template
    - name: "vote-monitor"
      accounts:
        - "burndao.burn"
        - "another.account"
      notify_operations:
        - "vote"
      operation_filters: {}
      message_template: |              # Rule-specific template
        🗳️ <b>New Vote Detected</b>

        <b>Voter:</b> <code>{{.Account}}</code>
        <b>Block:</b> <code>{{.BlockNum}}</code>

        <b>Details:</b>
        {{.Details}}
```

#### Configuration Options

**Global Settings:**
- `enabled`: Enable/disable Telegram notifications
- `bot_token`: Telegram bot token (required for all rules)
- `channel_id`: Telegram channel ID (required for all rules)
- `message_template`: Fallback template used when rules don't define their own

**Rule Settings (each rule in `users` array):**
- `name`: Rule identifier (for logging)
- `accounts`: List of accounts to monitor (empty = all tracked accounts)
- `notify_operations`: List of operation types to notify (empty = all types)
- `operation_filters`: Operation-specific filters
  - `transfer.ignore_to_addresses`: Whitelist of addresses to ignore
- `message_template`: Optional rule-specific template (overrides global)

#### Template Variables

Available variables in message templates:
- `{{.Account}}`: Account name
- `{{.OpType}}`: Operation type
- `{{.BlockNum}}`: Block number
- `{{.Timestamp}}`: Operation timestamp
- `{{.Details}}`: Formatted operation details

#### Backward Compatibility

The legacy configuration format is still fully supported. If the `users` field is empty or not present, the system will automatically convert the legacy format to a single rule named "default".

## Building

### Local Development

#### Go Services

```bash
# Build sync service
go build -o sync ./cmd/sync

# Build API service
go build -o api ./cmd/api

# Build compensator tool
go build -o compensator ./cmd/compensator
```

#### Frontend

```bash
cd web
pnpm install
pnpm run dev
```

### Docker

```bash
docker build -t sps-fund-watcher .
docker run -d \
  -p 80:80 \
  -v $(pwd)/configs/config.yaml:/app/configs/config.yaml \
  sps-fund-watcher
```

## API Endpoints

- `GET /api/v1/health` - Health check
- `GET /api/v1/accounts` - List all tracked accounts
- `GET /api/v1/accounts/:account/operations` - Get operations for an account
  - Query params: `page`, `page_size`, `type` (optional operation type filter)
- `GET /api/v1/accounts/:account/transfers` - Get transfer operations only
- `GET /api/v1/accounts/:account/updates` - Get account update operations

## Web Interface

The web interface is available at `http://localhost` (when running in Docker) or `http://localhost:5173` (when running `pnpm run dev`).

Features:
- Account selector (defaults to `burndao.burn`)
- Operation type filter (All, Transfers, Account Updates)
- Paginated operation table
- Responsive design

## Running Services

### Starting Sync Service

The sync service reads configuration from a YAML file. **Important**: Always use the `-config` flag when starting the service:

```bash
go run cmd/sync/main.go -config configs/config.yaml
```

Or with a custom config file:

```bash
go run cmd/sync/main.go -config configs/config.temp.yaml
```

**Note**: Without the `-config` flag, the service will use the default config file path.

### Starting API Service

```bash
go run cmd/api/main.go -config configs/config.yaml
```

### Using Compensator Tool

The compensator tool is used to fetch historical operations for accounts that were added to tracking after the sync service has already been running. This fills the gap for operations that occurred before the account was added to the tracking list.

**Usage:**

```bash
./compensator -account <account_name> -start <start_block> -end <end_block> <config_file>
```

**Example:**

```bash
./compensator -account burndao.burn -start 101777000 -end 101780000 configs/config.yaml
```

**Parameters:**
- `-account`: The account name to fetch operations for (required)
- `-start`: Starting block number (required, must be > 0)
- `-end`: Ending block number (required, must be > 0, must be >= start)
- `config_file`: Path to configuration file (required, positional argument)

**What it does:**
1. Loads configuration from the specified YAML file
2. Connects to Steem API using `steem.api_url` from config
3. Connects to MongoDB using `mongodb.uri` and `mongodb.database` from config
4. Fetches blocks from `start` to `end` in batches (using `steem.batch_size` from config)
5. Extracts and stores all operations for the specified account in that range
6. Uses upsert to prevent duplicate operations

**Note:** The compensator does not update sync state or send Telegram notifications, as it's designed for historical data only.

### Resetting Sync State

If you need to restart synchronization from a specific block height, you can clear the sync state:

**Option 1: Clear sync state only (keeps existing operations)**
```bash
docker exec sps-fund-watcher-mongo-temp mongo sps_fund_watcher --eval "db.sync_state.drop(); print('Sync state dropped')" --quiet
```

**Option 2: Clear entire database (removes all operations and sync state)**
```bash
docker exec sps-fund-watcher-mongo-temp mongo sps_fund_watcher --eval "db.dropDatabase(); print('Database dropped')" --quiet
```

After clearing the sync state, restart the sync service with your desired configuration. The service will start from the `start_block` specified in your config file.

## Development

### Prerequisites

- Go 1.21+
- Node.js 20+
- pnpm (for frontend package management)
- MongoDB
- Docker (for containerized deployment)

### Project Structure

```
sps-fund-watcher/
├── cmd/
│   ├── sync/          # Sync service entry point
│   ├── compensator/   # Compensator tool entry point
│   └── api/            # API service entry point
├── internal/
│   ├── sync/           # Sync service logic
│   ├── api/            # API handlers and routes
│   ├── models/         # Data models
│   ├── storage/        # MongoDB storage layer
│   └── telegram/       # Telegram notification client
├── web/                # Frontend React app
│   ├── src/
│   ├── public/
│   └── package.json
├── configs/
│   ├── config.yaml     # Main configuration file
│   ├── supervisord.conf
│   └── nginx.conf
├── Dockerfile
└── README.md
```

## License

MIT
