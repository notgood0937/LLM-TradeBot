# LLM-TradeBot

当前仓库已切换为 Go 运行时，提供 Web 仪表盘、LLM 决策、Binance 接入、Telegram 通知，以及测试/实盘执行模式。

## 当前启动方式

- 默认启动：`./start.sh`
- Go 直接启动：`./start_go.sh`
- 仪表盘地址：`http://127.0.0.1:8000`

## 快速开始

```bash
cp .env.example .env
./install.sh
./start.sh
```

## 必要环境变量

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

## 主要目录

- `cmd/tradebotd`
  - Go 仪表盘后端
- `cmd/tradebot`
  - Go 命令行入口
- `internal/binance`
  - Binance Futures REST 客户端
- `internal/llm`
  - LLM 接入层
- `internal/execution`
  - 执行引擎
- `internal/server`
  - 仪表盘 API 与运行时编排
- `internal/state`
  - 共享状态、持仓、交易历史、权益曲线

## 编译

```bash
go build ./cmd/tradebot ./cmd/tradebotd
```

## Docker

```bash
docker build -t llm-tradebot .
docker run --rm -p 8000:8000 --env-file .env llm-tradebot
```

## 当前状态

仓库中的 Python 项目代码已经移除，当前有效运行时为 Go。
