#!/bin/bash

set -e

echo "============================================"
echo "🚀 Switch To Production"
echo "============================================"
echo ""

if [ ! -f ".env" ]; then
    echo "❌ .env not found"
    exit 1
fi

if grep -q '^BINANCE_TESTNET=' .env; then
    sed -i.bak 's/^BINANCE_TESTNET=.*/BINANCE_TESTNET=false/' .env
else
    echo "BINANCE_TESTNET=false" >> .env
fi

echo "✅ BINANCE_TESTNET=false"
echo "ℹ️  Review your .env carefully before running live"
echo "ℹ️  Start with: ./start.sh"
