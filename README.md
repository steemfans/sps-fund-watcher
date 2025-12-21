# SPS Fund Watcher

A comprehensive system for monitoring Steem blockchain operations for tracked accounts, with real-time Telegram notifications and a modern web interface.

## Features

- **Blockchain Sync Service**: Continuously syncs Steem blockchain data from a configured block height to the latest irreversible block
- **Account Tracking**: Monitor specific Steem accounts for all operations
- **Telegram Notifications**: Receive formatted notifications for selected operation types
- **REST API**: Full-featured API for querying operation history
- **Web Interface**: Modern React-based UI with pagination and filtering
- **Docker Deployment**: Single container deployment with supervisord and nginx

## Architecture

The system consists of three main components:

1. **Sync Service** (`cmd/sync`): Syncs blockchain data and stores it in MongoDB
2. **API Service** (`cmd/api`): Provides REST API endpoints for the web frontend
3. **Web Frontend** (`web/`): React application built with Vite, Tailwind CSS, and shadcn/ui

All services run in a single Docker container managed by supervisord.

## Configuration

Edit `configs/config.yaml` to configure:

```yaml
steem:
  api_url: "https://api.steem.fans"  # Steem API endpoint
  start_block: 50000000              # Starting block height
  accounts:
    - "burndao.burn"                 # Accounts to track

mongodb:
  uri: "mongodb://localhost:27017"   # MongoDB connection string
  database: "sps_fund_watcher"        # Database name

telegram:
  enabled: true                       # Enable Telegram notifications
  bot_token: ""                       # Telegram bot token
  channel_id: ""                      # Telegram channel ID
  notify_operations:                  # Operation types to notify
    - "transfer"
    - "account_update"
    - "account_update2"
    # Empty list means notify all operations

api:
  port: "8080"                        # API server port
  host: "0.0.0.0"                     # API server host
```

## Building

### Local Development

#### Go Services

```bash
# Build sync service
go build -o sync ./cmd/sync

# Build API service
go build -o api ./cmd/api
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
