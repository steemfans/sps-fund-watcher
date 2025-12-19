# SPS Fund Watcher 系统实现计划

## 架构概述

系统包含三个主要组件：
- **同步服务 (Sync Service)**: 从配置的区块高度开始，持续同步 Steem 区块链数据到最新不可逆高度
- **API 服务 (API Service)**: 为 Web 前端提供 REST API 接口
- **Web 前端**: 基于 React 的 UI 界面，展示追踪账号的数据

所有服务运行在单个 Docker 容器中，由 supervisord 管理，nginx 提供前端服务和 API 代理。

## 项目结构

```
sps-fund-watcher/
├── cmd/
│   ├── sync/          # 同步服务入口
│   └── api/            # API 服务入口
├── internal/
│   ├── sync/           # 同步服务逻辑
│   ├── api/            # API 处理器和路由
│   ├── models/         # 数据模型
│   ├── storage/        # MongoDB 存储层
│   └── telegram/       # Telegram 通知客户端
├── web/                # 前端 React 应用
│   ├── src/
│   ├── public/
│   └── package.json
├── configs/
│   ├── config.yaml     # 主配置文件
│   ├── supervisord.conf
│   └── nginx.conf
├── Dockerfile
├── go.mod
└── README.md
```

## 实现细节

### 1. Go 同步服务 (`cmd/sync/main.go`)

- 从 YAML 文件读取配置
- 初始化 MongoDB 连接
- 初始化 Telegram 客户端（如果已配置）
- 使用 steemgosdk 连接 Steem API（默认：https://api.steem.fans）
- 从配置的区块高度开始
- 持续获取区块直到最新不可逆高度
- 按追踪账号过滤操作
- 将操作存储到 MongoDB
- 在数据入库时发送格式化的 Telegram 通知（根据配置的 operation 类型过滤）

**关键组件：**
- `internal/sync/syncer.go`: 主同步循环逻辑
- `internal/sync/block_processor.go`: 处理单个区块和操作
- `internal/storage/mongodb.go`: MongoDB 操作（插入、查询）
- `internal/telegram/client.go`: Telegram Bot API 客户端

### 2. Go API 服务 (`cmd/api/main.go`)

- 初始化 Gin 路由器
- 连接 MongoDB
- 提供 REST 端点：
  - `GET /api/v1/accounts/:account/operations` - 列出账号的操作（分页）
  - `GET /api/v1/accounts/:account/transfers` - 仅列出转账操作
  - `GET /api/v1/accounts/:account/updates` - 列出账号更新操作
  - `GET /api/v1/accounts` - 列出所有追踪的账号
  - `GET /api/v1/health` - 健康检查

**关键组件：**
- `internal/api/handlers.go`: 请求处理器
- `internal/api/routes.go`: 路由定义
- `internal/models/operation.go`: 操作数据结构

### 3. 配置文件 (`configs/config.yaml`)

```yaml
steem:
  api_url: "https://api.steem.fans"
  start_block: 50000000
  accounts:
    - "burndao.burn"

mongodb:
  uri: "mongodb://localhost:27017"
  database: "sps_fund_watcher"

telegram:
  enabled: true
  bot_token: ""
  channel_id: ""
  # 配置需要推送的 operation 类型
  # 如果为空或未配置，则推送所有 operation 类型
  notify_operations:
    - "transfer"
    - "account_update"
    - "account_update2"

api:
  port: 8080
  host: "0.0.0.0"
```

**Telegram 推送配置说明：**
- `notify_operations`: 字符串数组，指定需要推送的 operation 类型
- 如果未配置或为空数组，则推送所有 operation 类型（默认行为）
- 如果配置了特定类型，只推送匹配的操作类型
- 所有 operation 都会存储到 MongoDB（无论是否推送）

### 4. 前端 (`web/`)

- 初始化 Vite + React + TypeScript 项目
- 安装和配置 Tailwind CSS
- 安装和配置 shadcn/ui
- 创建主页面组件，包含：
  - 账号选择器（默认：burndao.burn）
  - 操作类型过滤器（全部/转账/账号更新）
  - 使用 shadcn Table 组件的分页数据表格
  - 显示：时间戳、账号、操作类型、操作详情
- 使用 React Query 或 SWR 进行数据获取
- 使用 Tailwind CSS 实现响应式设计

**关键文件：**
- `web/src/App.tsx`: 主应用组件
- `web/src/components/OperationTable.tsx`: 数据表格组件
- `web/src/components/AccountSelector.tsx`: 账号过滤组件
- `web/src/api/client.ts`: 后端 API 调用客户端

### 5. Docker 设置

**Dockerfile:**
- 基础镜像：`alpine:3.19`（固定版本）
- 安装：Go、Node.js、nginx、supervisord、mongodb 客户端工具
- 构建 Go 服务和 React 前端
- 复制配置文件
- 设置 supervisord 管理：
  - sync 服务
  - api 服务
  - nginx
- 暴露端口 80

**supervisord.conf:**
- 配置 sync、api 和 nginx 的程序
- 设置日志和自动重启策略

**nginx.conf:**
- 从 `/web/dist` 提供静态文件
- 将 `/api/*` 请求代理到 API 服务（localhost:8080）
- 默认路由到 `index.html` 用于 SPA 路由

### 6. 数据模型

**MongoDB 集合：**
- `operations`: 存储所有追踪的操作
  - 字段：block_num, trx_id, account, op_type, op_data, timestamp, created_at
- `sync_state`: 跟踪同步进度
  - 字段：last_block, last_irreversible_block, updated_at

## 依赖项

**Go:**
- `github.com/gin-gonic/gin` - Web 框架
- `github.com/steem-go/steemgosdk` - Steem 区块链 SDK
- `go.mongodb.org/mongo-driver` - MongoDB 驱动
- `gopkg.in/yaml.v3` - YAML 配置解析

**前端:**
- `react`, `react-dom`
- `vite`
- `tailwindcss`
- `@radix-ui/react-*` (通过 shadcn/ui)
- `axios` 或 `fetch` 用于 API 调用

## 实现顺序

1. 设置 Go 项目结构和依赖
2. 实现 MongoDB 存储层
3. 实现带 steemgosdk 的同步服务
4. 实现 Telegram 通知客户端（支持 operation 类型过滤）
5. 实现带 Gin 的 API 服务
6. 设置前端项目（Vite + React + Tailwind + shadcn）
7. 构建前端组件和 API 集成
8. 创建 Dockerfile 和配置文件
9. 测试端到端功能

## Telegram 推送 Operation 类型过滤实现细节

### 配置结构更新

在 `internal/models/config.go` 中：
- 在 `TelegramConfig` 结构体中添加 `NotifyOperations []string` 字段
- 如果为空或 nil，则推送所有 operation 类型（向后兼容）

### 同步服务更新

在 `internal/sync/block_processor.go` 中：
- 在处理 operation 时，检查 operation 类型是否在 `notify_operations` 列表中
- 使用字符串匹配或映射来高效检查
- 只有匹配的 operation 类型才发送 Telegram 通知
- 所有 operation 仍然会存储到 MongoDB（无论是否推送）

### 默认行为

- 如果 `notify_operations` 未配置或为空数组，推送所有 operation 类型
- 如果配置了特定类型，只推送匹配的类型
- 配置示例支持常见的 operation 类型：transfer, account_update, account_update2 等

