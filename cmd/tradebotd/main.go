package main

import (
	"fmt"
	"log"
	"os"

	"llmtradebot/internal/config"
	"llmtradebot/internal/server"
)

func main() {
	cfg, err := config.Load(".env")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("startup: loading Go runtime")
	log.Printf("config: host=%s port=%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("config: symbols=%v primary=%s leverage=%d", cfg.Trading.Symbols, cfg.Trading.Primary, cfg.Trading.Leverage)
	log.Printf("config: llm_provider=%s model=%s llm_ready=%t", cfg.LLM.Provider, cfg.LLM.Model, cfg.LLM.APIKey != "" && cfg.LLM.BaseURL != "" && cfg.LLM.Model != "")
	log.Printf("config: telegram_enabled=%t telegram_ready=%t", cfg.Telegram.Enabled, cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "")
	log.Printf("config: binance_base=%s testnet=%t binance_ready=%t", cfg.Binance.BaseURL, cfg.Binance.Testnet, cfg.Binance.APIKey != "" && cfg.Binance.APISecret != "")

	srv := server.New(cfg)
	log.Printf("server: tradebotd listening on %s:%s", cfg.Server.Host, cfg.Server.Port)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
		os.Exit(1)
	}
}
