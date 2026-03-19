#!/bin/bash

set -e

echo "============================================"
echo "🚀 LLM-TradeBot Go Startup"
echo "============================================"
echo ""

if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go is not installed"
    exit 1
fi

if [ -f ".env" ]; then
    echo "ℹ️  Loading .env"
    set -a
    source .env
    set +a
else
    echo "⚠️  .env not found, using current environment"
fi

echo "ℹ️  Starting Go dashboard on http://127.0.0.1:8000"
exec go run ./cmd/tradebotd
