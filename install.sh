#!/bin/bash

set -e

echo "============================================"
echo "🤖 LLM-TradeBot Installation"
echo "============================================"
echo ""

if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go is not installed"
    echo "Please install Go 1.22+ first"
    exit 1
fi

echo "ℹ️  Go version: $(go version)"

if [ ! -f ".env" ] && [ -f ".env.example" ]; then
    cp .env.example .env
    echo "⚠️  Created .env from .env.example"
fi

mkdir -p runtime logs data models

echo "ℹ️  Building Go binaries..."
go build ./cmd/tradebot ./cmd/tradebotd

echo ""
echo "✅ Installation complete"
echo "Next steps:"
echo "  1. Edit .env"
echo "  2. Run: ./start.sh"
