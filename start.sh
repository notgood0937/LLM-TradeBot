#!/bin/bash
# ============================================
# LLM-TradeBot 一键启动脚本
# ============================================

set -e  # Exit on error

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print functions
print_info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
print_success() { echo -e "${GREEN}✅ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
print_error() { echo -e "${RED}❌ $1${NC}"; }

# Banner
echo "============================================"
echo "🚀 LLM-TradeBot Startup"
echo "============================================"
echo ""

# Check if virtual environment exists
VENV_DIR="venv"
if [ ! -d "$VENV_DIR" ]; then
    print_error "Virtual environment not found"
    print_info "Please run ./install.sh first"
    exit 1
fi

# Activate virtual environment
print_info "Activating virtual environment..."
source "$VENV_DIR/bin/activate"

# Check .env file (preferred)
if [ ! -f ".env" ]; then
    print_warning ".env file not found - will rely on config.yaml"
else
    # Check required environment variables
    print_info "Checking environment variables..."
    source .env

    MISSING_VARS=()
    [ -z "$BINANCE_API_KEY" ] && MISSING_VARS+=("BINANCE_API_KEY")
    [ -z "$BINANCE_SECRET_KEY" ] && MISSING_VARS+=("BINANCE_SECRET_KEY")
    [ -z "$DEEPSEEK_API_KEY" ] && MISSING_VARS+=("DEEPSEEK_API_KEY")
    [ -z "$TELEGRAM_BOT_TOKEN" ] && MISSING_VARS+=("TELEGRAM_BOT_TOKEN")
    [ -z "$TELEGRAM_CHAT_ID" ] && MISSING_VARS+=("TELEGRAM_CHAT_ID")

    if [ ${#MISSING_VARS[@]} -gt 0 ]; then
        print_warning "Missing environment variables in .env:"
        for var in "${MISSING_VARS[@]}"; do
            echo "  - $var"
        done
        print_info "Continuing anyway (will fallback to config.yaml where applicable)"
    else
        print_success "Environment variables OK"
    fi
fi

# Parse arguments
MODE_OVERRIDE=""
RUN_MODE_ARG=""
EXTRA_ARGS=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --test)
            MODE_OVERRIDE="test"
            shift
            ;;
        --live)
            MODE_OVERRIDE="live"
            shift
            ;;
        --mode)
            RUN_MODE_ARG="--mode $2"
            shift 2
            ;;
        *)
            EXTRA_ARGS="$EXTRA_ARGS $1"
            shift
            ;;
    esac
done

# Determine test/live mode
MODE_FLAG=""
if [ "$MODE_OVERRIDE" = "test" ]; then
    MODE_FLAG="--test"
elif [ "$MODE_OVERRIDE" = "live" ]; then
    MODE_FLAG="--live"
else
    ENV_RUN_MODE=$(echo "${RUN_MODE:-test}" | tr '[:upper:]' '[:lower:]')
    if [ "$ENV_RUN_MODE" = "live" ]; then
        MODE_FLAG="--live"
    else
        MODE_FLAG="--test"
    fi
fi

if [ -z "$RUN_MODE_ARG" ]; then
    RUN_MODE_ARG="--mode continuous"
fi

# Start the application
echo ""
print_info "Starting LLM-TradeBot..."
if [ "$MODE_FLAG" = "--test" ]; then
    print_info "Environment: TEST"
else
    print_info "Environment: LIVE"
fi
print_info "Run Mode: $RUN_MODE_ARG"
echo ""
print_success "Dashboard will be available at: http://localhost:8000"
echo ""

# Run main.py
python main.py $MODE_FLAG $RUN_MODE_ARG $EXTRA_ARGS
