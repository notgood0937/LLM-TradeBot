package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Binance  BinanceConfig
	LLM      LLMConfig
	Telegram TelegramConfig
	Server   ServerConfig
	Trading  TradingConfig
	Risk     RiskConfig
}

type BinanceConfig struct {
	APIKey    string
	APISecret string
	BaseURL   string
	Testnet   bool
	Timeout   time.Duration
}

type ServerConfig struct {
	Host string
	Port string
}

type TradingConfig struct {
	Symbols  []string
	Primary  string
	Leverage int
}

type RiskConfig struct {
	MaxRiskPerTradePct  float64
	MaxTotalPositionPct float64
	MaxLeverage         float64
	StopTradingDrawdown float64
}

type LLMConfig struct {
	Provider    string
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	MaxTokens   int
	Timeout     time.Duration
}

type TelegramConfig struct {
	Enabled  bool
	BotToken string
	ChatID   string
	Timeout  time.Duration
}

func Load(dotEnvPath string) (Config, error) {
	_ = loadDotEnv(dotEnvPath)

	timeout := 120 * time.Second
	tgTimeout := 10 * time.Second
	cfg := Config{
		Binance: BinanceConfig{
			APIKey:    os.Getenv("BINANCE_API_KEY"),
			APISecret: firstNonEmpty(os.Getenv("BINANCE_SECRET_KEY"), os.Getenv("BINANCE_API_SECRET")),
			BaseURL:   firstNonEmpty(os.Getenv("BINANCE_BASE_URL"), chooseBinanceBaseURL()),
			Testnet:   isTruthy(firstNonEmpty(os.Getenv("BINANCE_TESTNET"), "true")),
			Timeout:   30 * time.Second,
		},
		LLM: LLMConfig{
			Provider:    firstNonEmpty(os.Getenv("LLM_PROVIDER"), "deepseek"),
			APIKey:      firstNonEmpty(os.Getenv("DEEPSEEK_API_KEY"), os.Getenv("OPENAI_API_KEY")),
			BaseURL:     firstNonEmpty(os.Getenv("LLM_BASE_URL"), "https://api.deepseek.com"),
			Model:       firstNonEmpty(os.Getenv("LLM_MODEL"), os.Getenv("DEEPSEEK_MODEL"), "deepseek-chat"),
			Temperature: 0.3,
			MaxTokens:   2000,
			Timeout:     timeout,
		},
		Telegram: TelegramConfig{
			Enabled:  isTruthy(os.Getenv("TELEGRAM_ENABLED")),
			BotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
			ChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
			Timeout:  tgTimeout,
		},
		Server: ServerConfig{
			Host: firstNonEmpty(os.Getenv("HOST"), "127.0.0.1"),
			Port: firstNonEmpty(os.Getenv("PORT"), "8000"),
		},
		Trading: TradingConfig{
			Symbols:  parseCSV(firstNonEmpty(os.Getenv("TRADING_SYMBOLS"), "BTCUSDT")),
			Primary:  firstNonEmpty(os.Getenv("TRADING_PRIMARY_SYMBOL"), "BTCUSDT"),
			Leverage: int(parseFloatDefault(firstNonEmpty(os.Getenv("LEVERAGE"), "5"), 5)),
		},
		Risk: RiskConfig{
			MaxRiskPerTradePct:  parseFloatDefault(firstNonEmpty(os.Getenv("MAX_RISK_PER_TRADE_PCT"), "1.5"), 1.5),
			MaxTotalPositionPct: parseFloatDefault(firstNonEmpty(os.Getenv("MAX_TOTAL_POSITION_PCT"), "33.3"), 33.3),
			MaxLeverage:         parseFloatDefault(firstNonEmpty(os.Getenv("MAX_LEVERAGE"), "5"), 5),
			StopTradingDrawdown: parseFloatDefault(firstNonEmpty(os.Getenv("STOP_TRADING_ON_DRAWDOWN_PCT"), "10"), 10),
		},
	}

	if cfg.LLM.Provider == "openai" && cfg.LLM.APIKey == "" {
		cfg.LLM.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	return cfg, nil
}

func chooseBinanceBaseURL() string {
	if isTruthy(firstNonEmpty(os.Getenv("BINANCE_TESTNET"), "true")) {
		return "https://testnet.binancefuture.com"
	}
	return "https://fapi.binance.com"
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"BTCUSDT"}
	}
	return out
}

func parseFloatDefault(raw string, fallback float64) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}
