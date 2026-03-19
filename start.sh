#!/bin/bash
# ============================================
# LLM-TradeBot 默认启动脚本（Go）
# ============================================

set -e

echo "============================================"
echo "🚀 LLM-TradeBot Startup"
echo "============================================"
echo ""
echo "ℹ️  Default runtime: Go"
echo "ℹ️  Legacy Python runtime is pending removal"
echo ""

exec ./start_go.sh "$@"
