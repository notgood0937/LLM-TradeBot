# Quick Start

## 1. Configure Environment

```bash
cp .env.example .env
```

Fill at least:

```bash
BINANCE_API_KEY=your_key
BINANCE_SECRET_KEY=your_secret
DEEPSEEK_API_KEY=your_key
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id
```

## 2. Install

```bash
./install.sh
```

## 3. Start

```bash
./start.sh
```

## 4. Open Dashboard

```text
http://127.0.0.1:8000
```

## 5. Build Manually

```bash
go build ./cmd/tradebot ./cmd/tradebotd
```

## 6. Start Go Runtime Directly

```bash
./start_go.sh
```
