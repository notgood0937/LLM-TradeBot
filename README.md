# LLM-TradeBot

Go-first trading bot runtime with web dashboard, LLM decision flow, Binance integration, Telegram notifications, and test/live execution modes.

## Current Runtime

- Default startup: `./start.sh`
- Go direct startup: `./start_go.sh`
- HTTP dashboard: `http://127.0.0.1:8000`

## Quick Start

```bash
cp .env.example .env
./install.sh
./start.sh
```

## Required Environment Variables

```bash
BINANCE_API_KEY=your_key
BINANCE_SECRET_KEY=your_secret
BINANCE_TESTNET=true

LLM_PROVIDER=deepseek
DEEPSEEK_API_KEY=your_key
LLM_MODEL=deepseek-chat
LLM_BASE_URL=https://api.deepseek.com

TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id
```

## Main Components

- `cmd/tradebotd`
  - Go dashboard/backend server
- `cmd/tradebot`
  - Go CLI entry
- `internal/binance`
  - Binance Futures REST client
- `internal/llm`
  - LLM provider integration
- `internal/execution`
  - Order execution engine
- `internal/server`
  - Dashboard API and runtime orchestration
- `internal/state`
  - Shared runtime state, trade history, positions, equity curve

## Build

```bash
go build ./cmd/tradebot ./cmd/tradebotd
```

## Docker

```bash
docker build -t llm-tradebot .
docker run --rm -p 8000:8000 --env-file .env llm-tradebot
```

## Status

Python project code has been removed from the repository. The active runtime is Go.
