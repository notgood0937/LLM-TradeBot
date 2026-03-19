package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"llmtradebot/internal/app"
	"llmtradebot/internal/binance"
	"llmtradebot/internal/config"
	"llmtradebot/internal/domain"
	"llmtradebot/internal/execution"
	"llmtradebot/internal/quant"
	"llmtradebot/internal/risk"
	"llmtradebot/internal/selector"
	"llmtradebot/internal/state"
)

type Server struct {
	cfg           config.Config
	binance       *binance.Client
	quant         *quant.Analyzer
	decision      *app.DecisionService
	risk          *risk.Auditor
	exec          *execution.Engine
	selector      *selector.Agent
	state         *state.SharedState
	stateFile     string
	promptFile    string
	customPrompt  string
	cycleInterval float64
	agentConfig   map[string]bool
	agentSettings map[string]any
	accounts      []map[string]any
	loopMu        sync.Mutex
	loopCancel    context.CancelFunc
}

func New(cfg config.Config) *Server {
	st := state.New(cfg.Trading.Symbols)
	stateFile := "runtime/state.json"
	promptFile := "runtime/custom_prompt.txt"
	_ = os.MkdirAll("runtime", 0o755)
	_ = st.Load(stateFile)
	promptBytes, _ := os.ReadFile(promptFile)

	client := binance.New(cfg.Binance)
	return &Server{
		cfg:           cfg,
		binance:       client,
		quant:         quant.New(),
		decision:      app.NewDecisionService(cfg),
		risk:          risk.New(),
		exec:          execution.New(client, cfg),
		selector:      selector.New(client, cfg.Trading.Symbols),
		state:         st,
		stateFile:     stateFile,
		promptFile:    promptFile,
		customPrompt:  string(promptBytes),
		cycleInterval: 3,
		agentConfig:   defaultAgentConfig(),
		agentSettings: defaultAgentSettings(),
		accounts:      []map[string]any{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/info", s.handleInfo)
	mux.HandleFunc("/api/login/default", s.handleLoginDefault)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/control", s.handleControl)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/config/default_prompt", s.handleDefaultPrompt)
	mux.HandleFunc("/api/config/prompt", s.handlePrompt)
	mux.HandleFunc("/api/upload_prompt", s.handleUploadPrompt)
	mux.HandleFunc("/api/state/save", s.handleSaveState)
	mux.HandleFunc("/api/state/load", s.handleLoadState)
	mux.HandleFunc("/api/market/klines", s.handleKlines)
	mux.HandleFunc("/api/account", s.handleAccount)
	mux.HandleFunc("/api/accounts", s.handleAccounts)
	mux.HandleFunc("/api/accounts/", s.handleAccountByID)
	mux.HandleFunc("/api/symbol_stats", s.handleSymbolStats)
	mux.HandleFunc("/api/agents/config", s.handleAgentConfig)
	mux.HandleFunc("/api/agents/settings", s.handleAgentSettings)
	mux.HandleFunc("/api/selector/run", s.handleSelectorRun)
	mux.HandleFunc("/api/decision", s.handleDecision)
	mux.HandleFunc("/api/pipeline/run", s.handlePipelineRun)
	mux.HandleFunc("/api/execute", s.handleExecute)
	mux.HandleFunc("/", s.handleStatic)
	return s.withLogging(mux)
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf("%s:%s", s.cfg.Server.Host, s.cfg.Server.Port)
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"binance_ready": s.binance.Ready(),
		"symbols":       s.cfg.Trading.Symbols,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"deployment_mode": "local",
		"requires_auth":   false,
	})
}

func (s *Server) handleLoginDefault(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"password": "EthanAlgoX"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "method not allowed"})
		return
	}
	log.Printf("auth: login success role=admin remote=%s", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "role": "admin"})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "success"})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "authenticated"})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	snap := s.state.Snapshot()
	agentMessages, _ := snap["agent_messages"].([]map[string]any)
	latestDecision, _ := snap["latest_decision"].(map[string]any)
	decisionHistory, _ := snap["decision_history"].([]map[string]any)
	positions, _ := snap["positions"].([]map[string]any)
	tradeHistory, _ := snap["trade_history"].([]map[string]any)
	chartData, _ := snap["chart_data"].(map[string]any)
	virtualAccount, _ := snap["virtual_account"].(map[string]any)
	selectorInfo, _ := snap["symbol_selector"].(map[string]any)
	system := map[string]any{
		"mode":             snap["execution_mode"],
		"running":          snap["is_running"],
		"is_test_mode":     snap["is_test_mode"],
		"cycle_counter":    snap["cycle_counter"],
		"cycle_interval":   s.cycleInterval,
		"symbols":          snap["symbols"],
		"current_symbol":   snap["current_symbol"],
		"timeframes":       []string{"5m", "15m", "1h"},
		"current_cycle_id": fmt.Sprintf("cycle_%04v", snap["cycle_counter"]),
		"uptime_start":     fmt.Sprint(snap["start_time"]),
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"system":           system,
		"market":           map[string]any{"prices": snap["current_price"]},
		"agents":           map[string]any{"symbol_selector": selectorInfo, "agent_messages": agentMessages},
		"decision":         latestDecision,
		"decision_history": decisionHistory,
		"logs":             snap["recent_logs"],
		"logs_simplified":  snap["recent_logs"],
		"account":          toAccountPayload(snap["account_overview"]),
		"positions":        positions,
		"virtual_account":  virtualAccount,
		"trade_history":    tradeHistory,
		"chart_data":       chartData,
		"account_alert":    map[string]any{"active": false, "failure_count": 0},
		"demo": map[string]any{
			"active":              false,
			"expired":             false,
			"remaining_seconds":   0,
			"demo_mode_active":    false,
			"demo_expired":        false,
			"demo_time_remaining": 0,
		},
	})
}

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req struct {
		Action   string  `json:"action"`
		Mode     string  `json:"mode"`
		Interval float64 `json:"interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	switch req.Action {
	case "start":
		log.Printf("control: action=start mode=%s", req.Mode)
		s.state.SetMode(true, "Running", req.Mode != "live")
		s.startLoop()
	case "pause":
		log.Printf("control: action=pause mode=%s", req.Mode)
		s.state.SetMode(false, "Paused", req.Mode != "live")
		s.stopLoop("pause")
	case "stop":
		log.Printf("control: action=stop mode=%s", req.Mode)
		s.state.SetMode(false, "Stopped", req.Mode != "live")
		s.stopLoop("stop")
	case "set_mode":
		isTest := req.Mode != "live"
		log.Printf("control: action=set_mode mode=%s is_test=%t", req.Mode, isTest)
		s.state.SetMode(s.state.Snapshot()["is_running"] == true, fmt.Sprint(s.state.Snapshot()["execution_mode"]), isTest)
		writeJSON(w, http.StatusOK, map[string]any{"status": "success", "is_test_mode": isTest})
		return
	case "set_interval":
		if req.Interval > 0 {
			s.cycleInterval = req.Interval
		}
		log.Printf("control: action=set_interval interval=%.2f", s.cycleInterval)
		if s.state.Snapshot()["is_running"] == true {
			log.Printf("loop: restarting to apply interval=%.2f", s.cycleInterval)
			s.stopLoop("interval_update")
			s.startLoop()
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "success", "cycle_interval": s.cycleInterval})
		return
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown action"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "success"})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"api_keys": map[string]any{
				"binance_api_key":    maskKey(s.cfg.Binance.APIKey),
				"binance_secret_key": maskKey(s.cfg.Binance.APISecret),
				"deepseek_api_key":   maskKey(s.cfg.LLM.APIKey),
				"openai_api_key":     maskKey(os.Getenv("OPENAI_API_KEY")),
				"claude_api_key":     maskKey(os.Getenv("CLAUDE_API_KEY")),
				"qwen_api_key":       maskKey(os.Getenv("QWEN_API_KEY")),
				"gemini_api_key":     maskKey(os.Getenv("GEMINI_API_KEY")),
				"kimi_api_key":       maskKey(os.Getenv("KIMI_API_KEY")),
				"minimax_api_key":    maskKey(os.Getenv("MINIMAX_API_KEY")),
				"glm_api_key":        maskKey(os.Getenv("GLM_API_KEY")),
			},
			"llm": map[string]any{
				"provider": s.cfg.LLM.Provider,
			},
			"trading": map[string]any{
				"run_mode": ternary(s.state.Snapshot()["is_test_mode"] == true, "test", "live"),
			},
		})
		return
	}

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if apiKeys, ok := req["api_keys"].(map[string]any); ok {
		if v, ok := apiKeys["binance_api_key"].(string); ok && v != "" {
			s.cfg.Binance.APIKey = v
		}
		if v, ok := apiKeys["binance_secret_key"].(string); ok && v != "" {
			s.cfg.Binance.APISecret = v
		}
		if v, ok := apiKeys["deepseek_api_key"].(string); ok && v != "" {
			s.cfg.LLM.APIKey = v
		}
	}
	if llmCfg, ok := req["llm"].(map[string]any); ok {
		if v, ok := llmCfg["llm_provider"].(string); ok && v != "" {
			s.cfg.LLM.Provider = v
		}
	}
	if tradingCfg, ok := req["trading"].(map[string]any); ok {
		if v, ok := tradingCfg["run_mode"].(string); ok {
			s.state.SetMode(s.state.Snapshot()["is_running"] == true, fmt.Sprint(s.state.Snapshot()["execution_mode"]), v != "live")
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "success"})
}

func (s *Server) handleDefaultPrompt(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"content": "You are the final trading decision engine. Return JSON only."})
}

func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"content": s.customPrompt})
	case http.MethodPost:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
			return
		}
		s.customPrompt = req.Content
		if err := os.WriteFile(s.promptFile, []byte(req.Content), 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "success"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "method not allowed"})
	}
}

func (s *Server) handleUploadPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}
	s.customPrompt = string(content)
	if err := os.WriteFile(s.promptFile, content, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	log.Printf("prompt: uploaded filename=%s size=%d", header.Filename, len(content))
	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "filename": header.Filename})
}

func (s *Server) handleSaveState(w http.ResponseWriter, _ *http.Request) {
	if err := s.state.Persist(s.stateFile); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved", "path": s.stateFile})
}

func (s *Server) handleLoadState(w http.ResponseWriter, _ *http.Request) {
	if err := s.state.Load(s.stateFile); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.state.Snapshot())
}

func (s *Server) handleKlines(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}
	candles, err := s.binance.GetKlines(ctx, symbol, interval, 200)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, candles)
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	account, err := s.binance.GetFuturesAccount(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	s.state.UpdateAccount(account.TotalMarginBalance, account.AvailableBalance, account.TotalWalletBalance, account.TotalUnrealizedProfit)
	if symbol := firstSymbol(s.cfg.Trading.Symbols); symbol != "" {
		if pos, posErr := s.binance.GetPosition(ctx, symbol); posErr == nil {
			s.state.UpdatePositions(buildPositionsPayload(pos))
		}
	}
	writeJSON(w, http.StatusOK, account)
}

func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"accounts": s.accounts})
	case http.MethodPost:
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
			return
		}
		acc := map[string]any{
			"id":            req["id"],
			"account_name":  req["name"],
			"exchange_type": req["exchange"],
			"testnet":       req["testnet"],
			"enabled":       true,
			"has_api_key":   s.cfg.Binance.APIKey != "",
		}
		s.accounts = append(s.accounts, acc)
		writeJSON(w, http.StatusOK, acc)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "method not allowed"})
	}
}

func (s *Server) handleAccountByID(w http.ResponseWriter, r *http.Request) {
	id := filepath.Base(r.URL.Path)
	if r.Method == http.MethodDelete {
		filtered := make([]map[string]any, 0, len(s.accounts))
		for _, acc := range s.accounts {
			if fmt.Sprint(acc["id"]) != id {
				filtered = append(filtered, acc)
			}
		}
		s.accounts = filtered
		writeJSON(w, http.StatusOK, map[string]any{"status": "success"})
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "method not allowed"})
}

func (s *Server) handleSymbolStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "data": []map[string]any{}})
}

func (s *Server) handleAgentConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"agents": s.agentConfig})
	case http.MethodPost:
		var req struct {
			Agents map[string]bool `json:"agents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
			return
		}
		for k, v := range req.Agents {
			s.agentConfig[k] = v
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "success", "agents": s.agentConfig})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "method not allowed"})
	}
}

func (s *Server) handleAgentSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"agents": s.agentSettings,
			"llm_info": map[string]any{
				"provider": s.cfg.LLM.Provider,
				"model":    s.cfg.LLM.Model,
			},
			"llm_metrics": map[string]any{
				"providers": map[string]any{
					s.cfg.LLM.Provider: map[string]any{
						"total_input_tokens":  0,
						"total_output_tokens": 0,
						"total_tokens":        0,
						"token_speed_tps":     0,
						"min_latency_ms":      0,
						"avg_latency_ms":      0,
						"max_latency_ms":      0,
					},
				},
			},
		})
	case http.MethodPost:
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
			return
		}
		if agents, ok := req["agents"].(map[string]any); ok {
			s.agentSettings = agents
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "success"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "method not allowed"})
	}
}

func (s *Server) handleSelectorRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	result, err := s.selector.SelectAuto1(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	s.state.UpdateSelector(selectorState(result))
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req app.DecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	decision, marketContext, err := s.decision.Run(ctx, req)
	extras := buildDecisionExtrasFromRequest(req, decision)
	log.Printf("decision: symbol=%s action=%s confidence=%.1f error=%v", req.Symbol, decision.Action, decision.Confidence, err)
	status := http.StatusOK
	if err != nil {
		status = http.StatusBadGateway
	}
	s.state.AddAgentMessage("decision_core", fmt.Sprintf("Action: %s | Conf: %.1f%%", decision.Action, decision.Confidence), "info", req.Symbol)
	s.state.UpdateDecision(req.Symbol, decisionToMap(decision, req.Symbol, extras))
	writeJSON(w, status, map[string]any{
		"decision":       decision,
		"market_context": marketContext,
		"error":          errString(err),
	})
}

func (s *Server) handlePipelineRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req struct {
		Symbol string `json:"symbol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Symbol == "" {
		req.Symbol = "BTCUSDT"
	}

	result, err := s.runPipelineOnce(r.Context(), req.Symbol)
	status := http.StatusOK
	if err != nil {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, result)
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req struct {
		Symbol   string          `json:"symbol"`
		Decision domain.Decision `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Symbol == "" {
		req.Symbol = s.cfg.Trading.Primary
	}
	log.Printf("execute: symbol=%s action=%s requested", req.Symbol, req.Decision.Action)
	if s.state.Snapshot()["is_test_mode"] == true {
		result := s.executeVirtual(req.Symbol, req.Decision)
		log.Printf("execute: symbol=%s action=%s success=%t mode=virtual message=%s", req.Symbol, req.Decision.Action, result.Success, result.Message)
		writeJSON(w, http.StatusOK, result)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	price, err := s.binance.GetTickerPrice(ctx, req.Symbol)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	s.state.UpdatePrice(req.Symbol, price.Price)
	account, err := s.binance.GetFuturesAccount(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	position, _ := s.binance.GetPosition(ctx, req.Symbol)
	result := s.exec.ExecuteDecision(ctx, req.Decision, account, position, price.Price, req.Symbol)
	s.state.AddLog("execution: " + result.Message)
	if refreshedAccount, accErr := s.binance.GetFuturesAccount(ctx); accErr == nil {
		s.state.UpdateAccount(refreshedAccount.TotalMarginBalance, refreshedAccount.AvailableBalance, refreshedAccount.TotalWalletBalance, refreshedAccount.TotalUnrealizedProfit)
		account = refreshedAccount
	}
	if refreshedPosition, posErr := s.binance.GetPosition(ctx, req.Symbol); posErr == nil {
		s.state.UpdatePositions(buildPositionsPayload(refreshedPosition))
		position = refreshedPosition
	}
	s.state.AppendTrade(buildTradeRecord(req.Symbol, req.Decision, result, price.Price, position, account))
	log.Printf("execute: symbol=%s action=%s success=%t mode=live message=%s", req.Symbol, req.Decision.Action, result.Success, result.Message)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "web/index.html")
		return
	}
	if r.URL.Path == "/login" {
		http.ServeFile(w, r, "web/login.html")
		return
	}
	path := filepath.Clean(r.URL.Path)
	if path == "" || path == "." || path == "/" {
		http.NotFound(w, r)
		return
	}
	if filepath.Dir(path) == "/static" {
		path = "/" + filepath.Base(path)
	}
	base := filepath.Base(path)
	full := filepath.Join("web", base)
	if _, err := os.Stat(full); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, full)
}

func positionToMap(pos *binance.Position) map[string]any {
	if pos == nil {
		return map[string]any{}
	}
	return map[string]any{
		"symbol":         pos.Symbol,
		"position_amt":   pos.PositionAmt,
		"entry_price":    pos.EntryPrice,
		"mark_price":     pos.MarkPrice,
		"unrealized_pnl": pos.UnrealizedProfit,
		"leverage":       pos.Leverage,
		"side":           positionSide(pos),
	}
}

func positionSide(pos *binance.Position) string {
	if pos == nil {
		return ""
	}
	switch {
	case pos.PositionAmt > 0:
		return "LONG"
	case pos.PositionAmt < 0:
		return "SHORT"
	default:
		return pos.PositionSide
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("http: method=%s path=%s status=%d duration=%s remote=%s", r.Method, r.URL.Path, lrw.status, time.Since(start).Round(time.Millisecond), r.RemoteAddr)
	})
}

func (s *Server) startLoop() {
	s.loopMu.Lock()
	defer s.loopMu.Unlock()
	if s.loopCancel != nil {
		log.Printf("loop: already running")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.loopCancel = cancel
	go s.runLoop(ctx)
}

func (s *Server) stopLoop(reason string) {
	s.loopMu.Lock()
	cancel := s.loopCancel
	s.loopCancel = nil
	s.loopMu.Unlock()
	if cancel != nil {
		log.Printf("loop: stopping reason=%s", reason)
		cancel()
	}
}

func (s *Server) runLoop(ctx context.Context) {
	log.Printf("loop: started interval=%.2fmin", s.cycleInterval)
	s.state.AddLog(fmt.Sprintf("loop started interval=%.2fmin", s.cycleInterval))
	defer func() {
		s.loopMu.Lock()
		s.loopCancel = nil
		s.loopMu.Unlock()
		log.Printf("loop: stopped")
		s.state.AddLog("loop stopped")
	}()

	runCycle := func() {
		symbol := s.pickCycleSymbol(ctx)
		if symbol == "" {
			symbol = firstSymbol(s.cfg.Trading.Symbols)
		}
		if symbol == "" {
			log.Printf("loop: skipped cycle no symbol")
			return
		}
		log.Printf("============================================================")
		log.Printf("cycle: start symbol=%s interval=%.2fmin", symbol, s.cycleInterval)
		log.Printf("loop: cycle start symbol=%s", symbol)
		s.state.AddLog("cycle start: " + symbol)
		result, err := s.runPipelineOnce(ctx, symbol)
		if err != nil {
			log.Printf("loop: cycle failed symbol=%s error=%v", symbol, err)
			s.state.AddLog("cycle failed: " + err.Error())
			log.Printf("cycle: end symbol=%s status=failed", symbol)
			log.Printf("============================================================")
			return
		}
		decisionMap, _ := result["decision"].(domain.Decision)
		riskResult, _ := result["risk_audit"].(risk.Result)
		log.Printf("loop: cycle decision symbol=%s action=%s conf=%.1f risk_passed=%t", symbol, decisionMap.Action, decisionMap.Confidence, riskResult.Passed)
		if riskResult.Passed {
			if _, err := s.executeCycleDecision(ctx, symbol, decisionMap); err != nil {
				log.Printf("loop: execute failed symbol=%s error=%v", symbol, err)
				s.state.AddLog("execute failed: " + err.Error())
			}
		} else {
			log.Printf("loop: execution blocked symbol=%s risk_level=%s reason=%s", symbol, riskResult.RiskLevel, riskResult.BlockedReason)
			s.state.AddLog("execution blocked: " + riskResult.BlockedReason)
		}
		log.Printf("cycle: end symbol=%s status=done action=%s", symbol, decisionMap.Action)
		log.Printf("============================================================")
	}

	runCycle()
	ticker := time.NewTicker(time.Duration(s.cycleInterval * float64(time.Minute)))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := s.state.Snapshot()
			if snap["is_running"] != true {
				log.Printf("loop: skip tick not running")
				continue
			}
			runCycle()
		}
	}
}

func (s *Server) pickCycleSymbol(ctx context.Context) string {
	if s.agentConfig["symbol_selector_agent"] {
		result, err := s.selector.SelectAuto1(ctx)
		if err == nil {
			state := selectorState(result)
			s.state.UpdateSelector(state)
			if symbol := fmt.Sprint(state["symbol"]); symbol != "" && symbol != "<nil>" {
				log.Printf("selector: picked symbol=%s mode=%s", symbol, result.Mode)
				s.state.AddAgentMessage("symbol_selector", "Picked "+symbol, "info", symbol)
				return symbol
			}
		} else {
			log.Printf("selector: error=%v", err)
			s.state.AddLog("selector failed: " + err.Error())
		}
	}
	snap := s.state.Snapshot()
	if symbol := fmt.Sprint(snap["current_symbol"]); strings.TrimSpace(symbol) != "" {
		return symbol
	}
	return firstSymbol(s.cfg.Trading.Symbols)
}

func (s *Server) runPipelineOnce(parent context.Context, symbol string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()

	tf5m, err := s.binance.GetKlines(ctx, symbol, "5m", 200)
	if err != nil {
		log.Printf("pipeline: symbol=%s stage=tf5m error=%v", symbol, err)
		return nil, err
	}
	tf15m, err := s.binance.GetKlines(ctx, symbol, "15m", 200)
	if err != nil {
		log.Printf("pipeline: symbol=%s stage=tf15m error=%v", symbol, err)
		return nil, err
	}
	tf1h, err := s.binance.GetKlines(ctx, symbol, "1h", 200)
	if err != nil {
		log.Printf("pipeline: symbol=%s stage=tf1h error=%v", symbol, err)
		return nil, err
	}

	price, err := s.binance.GetTickerPrice(ctx, symbol)
	if err != nil {
		log.Printf("pipeline: symbol=%s stage=price error=%v", symbol, err)
		return nil, err
	}

	s.state.UpdatePrice(symbol, price.Price)

	account, accountErr := s.binance.GetFuturesAccount(ctx)
	position, posErr := s.binance.GetPosition(ctx, symbol)
	s.state.UpdatePositions(buildPositionsPayload(position))
	if accountErr == nil {
		s.state.UpdateAccount(account.TotalMarginBalance, account.AvailableBalance, account.TotalWalletBalance, account.TotalUnrealizedProfit)
	}

	analysis := s.quant.Analyze(tf1h, tf15m, tf5m)
	log.Printf("quant: symbol=%s trend=%.1f osc=%.1f sentiment=%.1f score=%.1f regime=%v",
		symbol,
		toFloat(analysis.Trend["total_trend_score"]),
		toFloat(analysis.Oscillator["total_osc_score"]),
		toFloat(analysis.Sentiment["total_sentiment_score"]),
		toFloat(analysis.Comprehensive["score"]),
		analysis.Regime["regime"],
	)
	s.state.AddAgentMessage("quant_analyst", "Analysis complete", "info", symbol)
	addSemanticAgentMessages(s.state, symbol, analysis.Semantic)

	decisionReq := app.DecisionRequest{
		Symbol:       symbol,
		CurrentPrice: price.Price,
		QuantAnalysis: map[string]any{
			"trend":         analysis.Trend,
			"oscillator":    analysis.Oscillator,
			"sentiment":     analysis.Sentiment,
			"comprehensive": analysis.Comprehensive,
		},
		PredictResult:     map[string]any{},
		ReflectionText:    "",
		CurrentPosition:   positionToMap(position),
		RegimeInfo:        analysis.Regime,
		SelectedAgentData: analysis.Semantic,
	}

	decision, marketContext, decisionErr := s.decision.Run(ctx, decisionReq)
	riskInput := risk.CheckInput{
		Decision:       decision,
		AccountBalance: account.AvailableBalance,
		CurrentPrice:   price.Price,
		HasPosition:    position != nil && position.PositionAmt != 0,
		PositionSide:   positionSide(position),
	}
	riskResult := s.risk.Audit(riskInput)
	s.state.AddAgentMessage("bull_agent", fmt.Sprintf("%s %d%% | %s", compactStance(decision.BullPerspective.Stance), decision.BullPerspective.Confidence, trimForPanel(decision.BullPerspective.Reasons)), "info", symbol)
	s.state.AddAgentMessage("bear_agent", fmt.Sprintf("%s %d%% | %s", compactStance(decision.BearPerspective.Stance), decision.BearPerspective.Confidence, trimForPanel(decision.BearPerspective.Reasons)), "info", symbol)
	s.state.AddAgentMessage("risk_audit", fmt.Sprintf("passed=%t level=%s reason=%s", riskResult.Passed, riskResult.RiskLevel, trimForPanel(firstWarningOrDefault(riskResult, "ok"))), "info", symbol)
	log.Printf("risk: symbol=%s passed=%t level=%s reason=%s warnings=%v",
		symbol,
		riskResult.Passed,
		riskResult.RiskLevel,
		riskResult.BlockedReason,
		riskResult.Warnings,
	)
	s.state.AddAgentMessage("decision_core", fmt.Sprintf("Action: %s | Conf: %.1f%%", decision.Action, decision.Confidence), "info", symbol)
	s.state.UpdateDecision(symbol, decisionToMap(decision, symbol, buildDecisionExtras(symbol, price.Price, analysis, riskResult, position, account.AvailableBalance)))
	log.Printf("pipeline: symbol=%s action=%s confidence=%.1f risk_passed=%t risk_level=%s", symbol, decision.Action, decision.Confidence, riskResult.Passed, riskResult.RiskLevel)

	result := map[string]any{
		"symbol":         symbol,
		"price":          price,
		"quant_analysis": analysis,
		"decision":       decision,
		"risk_audit":     riskResult,
		"market_context": marketContext,
		"account_error":  errString(accountErr),
		"position_error": errString(posErr),
		"decision_error": errString(decisionErr),
	}
	if decisionErr != nil {
		return result, decisionErr
	}
	return result, nil
}

func (s *Server) executeCycleDecision(ctx context.Context, symbol string, decision domain.Decision) (execution.Result, error) {
	if strings.EqualFold(decision.Action, "wait") || strings.EqualFold(decision.Action, "hold") || strings.TrimSpace(decision.Action) == "" {
		s.state.AddAgentMessage("execution", "Passive action, no order sent", "info", symbol)
		return execution.Result{Success: true, Action: decision.Action, Message: "passive action"}, nil
	}
	reqBody := struct {
		Symbol   string
		Decision domain.Decision
	}{Symbol: symbol, Decision: decision}
	_ = reqBody

	snap := s.state.Snapshot()
	if snap["is_test_mode"] == true {
		result := s.executeVirtual(symbol, decision)
		log.Printf("execute: symbol=%s action=%s success=%t mode=virtual message=%s", symbol, decision.Action, result.Success, result.Message)
		s.state.AddAgentMessage("execution", trimForPanel(result.Message), ternary(result.Success, "info", "error"), symbol)
		return result, nil
	}
	execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	price, err := s.binance.GetTickerPrice(execCtx, symbol)
	if err != nil {
		return execution.Result{}, err
	}
	s.state.UpdatePrice(symbol, price.Price)
	account, err := s.binance.GetFuturesAccount(execCtx)
	if err != nil {
		return execution.Result{}, err
	}
	position, _ := s.binance.GetPosition(execCtx, symbol)
	result := s.exec.ExecuteDecision(execCtx, decision, account, position, price.Price, symbol)
	s.state.AddLog("execution: " + result.Message)
	if refreshedAccount, accErr := s.binance.GetFuturesAccount(execCtx); accErr == nil {
		s.state.UpdateAccount(refreshedAccount.TotalMarginBalance, refreshedAccount.AvailableBalance, refreshedAccount.TotalWalletBalance, refreshedAccount.TotalUnrealizedProfit)
		account = refreshedAccount
	}
	if refreshedPosition, posErr := s.binance.GetPosition(execCtx, symbol); posErr == nil {
		s.state.UpdatePositions(buildPositionsPayload(refreshedPosition))
		position = refreshedPosition
	}
	s.state.AppendTrade(buildTradeRecord(symbol, decision, result, price.Price, position, account))
	log.Printf("execute: symbol=%s action=%s success=%t mode=live message=%s", symbol, decision.Action, result.Success, result.Message)
	s.state.AddAgentMessage("execution", trimForPanel(result.Message), ternary(result.Success, "info", "error"), symbol)
	if !result.Success {
		return result, fmt.Errorf(result.Message)
	}
	return result, nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func toAccountPayload(raw any) map[string]any {
	m, _ := raw.(map[string]float64)
	initialBalance := m["wallet_balance"]
	if initialBalance <= 0 {
		initialBalance = m["total_equity"] - m["total_pnl"]
	}
	if initialBalance < 0 {
		initialBalance = 0
	}
	return map[string]any{
		"total_equity":      m["total_equity"],
		"available_balance": m["available_balance"],
		"wallet_balance":    m["wallet_balance"],
		"total_pnl":         m["total_pnl"],
		"initial_balance":   initialBalance,
	}
}

func decisionToMap(d domain.Decision, symbol string, extras map[string]any) map[string]any {
	payload := map[string]any{
		"symbol":     symbol,
		"action":     d.Action,
		"confidence": d.Confidence,
		"reason":     d.Reasoning,
		"reasoning":  d.Reasoning,
		"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
		"vote_details": map[string]any{
			"bull_confidence": d.BullPerspective.Confidence,
			"bear_confidence": d.BearPerspective.Confidence,
			"bull_stance":     d.BullPerspective.Stance,
			"bear_stance":     d.BearPerspective.Stance,
		},
		"prophet_probability": 0.5,
		"guardian_passed":     true,
		"risk_level":          "safe",
	}
	for k, v := range extras {
		payload[k] = v
	}
	return payload
}

func buildDecisionExtras(symbol string, price float64, analysis quant.Analysis, riskResult risk.Result, position *binance.Position, availableBalance float64) map[string]any {
	trend1h, _ := analysis.Trend["trend_1h_score"].(float64)
	trend15m, _ := analysis.Trend["trend_15m_score"].(float64)
	trend5m, _ := analysis.Trend["trend_5m_score"].(float64)
	regime := cloneMap(analysis.Regime)
	regimeName := fmt.Sprint(regime["regime"])
	confidence := normalizedConfidence(analysis.Comprehensive["score"])
	positionInfo := derivePositionInfo(price, analysis)
	triggerPattern, triggerRVOL := deriveTrigger(analysis)

	return map[string]any{
		"cycle_number": currentCycleNumber(),
		"four_layer_status": map[string]any{
			"layer1_pass":      absFloat(trend1h) >= 40,
			"layer2_pass":      absFloat(trend15m) >= 15,
			"layer3_pass":      regimeName != "",
			"layer4_pass":      triggerRVOL >= 1.05,
			"trend_1h":         stanceFromScore(trend1h),
			"trend_15m":        stanceFromScore(trend15m),
			"trigger_pattern":  triggerPattern,
			"oi_change":        0.0,
			"regime":           regimeName,
			"multi_tf_aligned": trendAlignment(trend1h, trend15m, trend5m),
		},
		"indicator_snapshot":   indicatorSnapshot(analysis),
		"regime":               regime,
		"prophet_probability":  prophetProbability(analysis),
		"guardian_passed":      riskResult.Passed,
		"guardian_reason":      riskResult.BlockedReason,
		"risk_level":           riskResult.RiskLevel,
		"pending":              false,
		"multi_period_aligned": trendAlignment(trend1h, trend15m, trend5m),
		"trigger_pattern":      triggerPattern,
		"trigger_rvol":         triggerRVOL,
		"position_zone":        positionInfo["location"],
		"position":             positionInfo,
		"order_params":         orderParams(symbol, price, availableBalance, positionInfo),
		"semantic_analyses":    semanticAnalyses(analysis),
		"ai_filter_passed":     riskResult.Passed,
		"ai_filter_reason":     firstWarningOrDefault(riskResult, "quant-and-risk consensus"),
		"ai_filter_signal":     stanceFromScore(analysis.Comprehensive["score"]),
		"ai_filter_confidence": confidence,
		"reflection": map[string]any{
			"count":    1,
			"trades":   ternary(position != nil && position.PositionAmt != 0, 1, 0),
			"win_rate": 50.0,
			"text":     fmt.Sprintf("%s regime with %s trigger profile.", regimeName, triggerPattern),
		},
	}
}

func buildDecisionExtrasFromRequest(req app.DecisionRequest, decision domain.Decision) map[string]any {
	quantTrend, _ := req.QuantAnalysis["trend"].(map[string]any)
	trend1h := toFloat(quantTrend["trend_1h_score"])
	trend15m := toFloat(quantTrend["trend_15m_score"])
	trend5m := toFloat(quantTrend["trend_5m_score"])
	regime := cloneMap(req.RegimeInfo)
	regimeName := fmt.Sprint(regime["regime"])
	if regimeName == "" || regimeName == "<nil>" {
		regimeName = "unknown"
	}
	if regime["regime"] == nil {
		regime["regime"] = regimeName
	}
	positionInfo := cloneMap(req.CurrentPosition)
	if len(positionInfo) == 0 {
		positionInfo = map[string]any{
			"location":     "mid_range",
			"position_pct": 50.0,
			"range_low":    req.CurrentPrice * 0.98,
			"range_high":   req.CurrentPrice * 1.02,
			"range_size":   req.CurrentPrice * 0.04,
		}
	}
	semantic := semanticAnalysesFromRequest(req.SelectedAgentData, regime)
	triggerPattern := deriveTriggerPattern(summaryFromAgent(req.SelectedAgentData, "trigger_agent"))
	if triggerPattern == "None" {
		triggerPattern = "Momentum Continuation"
	}
	pUp := predictProbability(req.PredictResult)
	confidence := decision.Confidence
	if confidence <= 0 {
		confidence = normalizedConfidence(req.QuantAnalysis["comprehensive"])
	}
	guardReason := ""
	riskLevel := "safe"
	guardPassed := true
	if strings.EqualFold(decision.Action, "wait") && strings.Contains(strings.ToLower(decision.Reasoning), "failed") {
		guardReason = decision.Reasoning
		riskLevel = "warning"
	}
	return map[string]any{
		"cycle_number": currentCycleNumber(),
		"four_layer_status": map[string]any{
			"layer1_pass":      absFloat(trend1h) >= 20,
			"layer2_pass":      absFloat(trend15m) >= 10 || absFloat(trend5m) >= 10,
			"layer3_pass":      regimeName != "unknown",
			"layer4_pass":      !strings.EqualFold(decision.Action, "wait"),
			"trend_1h":         stanceFromScore(trend1h),
			"trend_15m":        stanceFromScore(trend15m),
			"trigger_pattern":  triggerPattern,
			"oi_change":        0.0,
			"regime":           regimeName,
			"multi_tf_aligned": trendAlignment(trend1h, trend15m, trend5m),
		},
		"indicator_snapshot": map[string]any{
			"ema_status":  stanceFromScore(trend1h),
			"rsi":         toFloat(req.QuantAnalysis["rsi"]),
			"macd_diff":   toFloat(req.QuantAnalysis["macd_diff"]),
			"bb_position": "middle",
		},
		"regime":               regime,
		"prophet_probability":  pUp,
		"guardian_passed":      guardPassed,
		"guardian_reason":      guardReason,
		"risk_level":           riskLevel,
		"pending":              false,
		"multi_period_aligned": trendAlignment(trend1h, trend15m, trend5m),
		"trigger_pattern":      triggerPattern,
		"trigger_rvol":         1.0,
		"position_zone":        positionInfo["location"],
		"position":             positionInfo,
		"order_params":         orderParams(req.Symbol, req.CurrentPrice, 1000, positionInfo),
		"semantic_analyses":    semantic,
		"ai_filter_passed":     true,
		"ai_filter_reason":     "request-path fallback",
		"ai_filter_signal":     decision.Action,
		"ai_filter_confidence": confidence,
		"reflection": map[string]any{
			"count":    1,
			"trades":   0,
			"win_rate": 50.0,
			"text":     firstNonEmptyString(req.ReflectionText, "No reflection provided"),
		},
	}
}

func selectorState(result selector.Result) map[string]any {
	state := map[string]any{
		"mode":    result.Mode,
		"symbol":  firstSymbol(result.Symbols),
		"symbols": result.Symbols,
		"scores":  result.Scores,
	}
	first := firstSymbol(result.Symbols)
	if scoreMap, ok := result.Scores[first].(map[string]any); ok {
		state["change_pct"] = scoreMap["change_pct"]
		state["volume_ratio"] = scoreMap["rvol"]
		state["score"] = scoreMap["score"]
		if change, ok := scoreMap["change_pct"].(float64); ok {
			switch {
			case change > 0.2:
				state["direction"] = "UP"
			case change < -0.2:
				state["direction"] = "DOWN"
			default:
				state["direction"] = "FLAT"
			}
		}
	}
	return state
}

func firstSymbol(symbols []string) string {
	if len(symbols) == 0 {
		return ""
	}
	return symbols[0]
}

func maskKey(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "******"
	}
	return v[:4] + "..." + v[len(v)-4:]
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func currentCycleNumber() int {
	return int(time.Now().Unix() % 100000)
}

func normalizedConfidence(v any) float64 {
	f, _ := v.(float64)
	conf := 50 + f/2
	if conf < 0 {
		return 0
	}
	if conf > 100 {
		return 100
	}
	return conf
}

func prophetProbability(analysis quant.Analysis) float64 {
	score, _ := analysis.Comprehensive["score"].(float64)
	p := 0.5 + score/200
	if p < 0.05 {
		return 0.05
	}
	if p > 0.95 {
		return 0.95
	}
	return p
}

func indicatorSnapshot(analysis quant.Analysis) map[string]any {
	details, _ := analysis.Trend["details"].(map[string]any)
	trend1h, _ := details["1h"].(map[string]any)
	osc1h, _ := analysis.Oscillator["oscillator_1h"].(map[string]any)
	oscDetails, _ := osc1h["details"].(map[string]any)
	rsiValue, _ := oscDetails["rsi_value"].(float64)
	closePrice, _ := trend1h["close"].(float64)
	ema20, _ := trend1h["ema20"].(float64)
	ema60, _ := trend1h["ema60"].(float64)
	return map[string]any{
		"ema_status":  trimAlignment(fmt.Sprint(trend1h["ema_status"])),
		"rsi":         rsiValue,
		"macd_diff":   ema20 - ema60,
		"bb_position": bandPosition(closePrice, ema20, ema60),
	}
}

func semanticAnalyses(analysis quant.Analysis) map[string]any {
	raw := cloneMap(analysis.Semantic)
	out := map[string]any{}
	if trend, ok := raw["trend_agent"].(map[string]any); ok {
		out["trend"] = map[string]any{
			"stance":   trend["stance"],
			"summary":  trend["summary"],
			"metadata": map[string]any{"strength": stanceStrength(fmt.Sprint(trend["stance"])), "adx": analysis.Regime["adx"]},
		}
	}
	if setup, ok := raw["setup_agent"].(map[string]any); ok {
		out["setup"] = setup
	}
	if trigger, ok := raw["trigger_agent"].(map[string]any); ok {
		_, rvol := deriveTrigger(analysis)
		out["trigger"] = map[string]any{
			"stance":  trigger["stance"],
			"summary": trigger["summary"],
			"metadata": map[string]any{
				"pattern": deriveTriggerPattern(fmt.Sprint(trigger["summary"])),
				"rvol":    rvol,
			},
		}
	}
	if multi, ok := raw["multi_period_agent"].(map[string]any); ok {
		out["multi_period"] = multi
	}
	return out
}

func deriveTrigger(analysis quant.Analysis) (string, float64) {
	triggerPattern := "No clear trigger"
	triggerRVOL := 1.0
	if sentiment, ok := analysis.Sentiment["total_sentiment_score"].(float64); ok {
		triggerRVOL += sentiment / 20
		if triggerRVOL < 0.5 {
			triggerRVOL = 0.5
		}
	}
	if trigger, ok := analysis.Semantic["trigger_agent"].(map[string]any); ok {
		triggerPattern = deriveTriggerPattern(fmt.Sprint(trigger["summary"]))
	}
	return triggerPattern, triggerRVOL
}

func deriveTriggerPattern(summary string) string {
	switch {
	case summary == "":
		return "None"
	case containsAny(summary, "breakout", "expansion"):
		return "Breakout"
	case containsAny(summary, "pullback", "retest"):
		return "Pullback"
	case containsAny(summary, "fade", "mean reversion"):
		return "Mean Reversion"
	default:
		return "Momentum Continuation"
	}
}

func derivePositionInfo(price float64, analysis quant.Analysis) map[string]any {
	details, _ := analysis.Trend["details"].(map[string]any)
	trend1h, _ := details["1h"].(map[string]any)
	ema20, _ := trend1h["ema20"].(float64)
	ema60, _ := trend1h["ema60"].(float64)
	low := minNonZero(ema20, ema60) * 0.995
	high := maxNonZero(ema20, ema60) * 1.005
	if low <= 0 || high <= 0 || high <= low {
		low = price * 0.98
		high = price * 1.02
	}
	pct := 50.0
	if high > low {
		pct = (price - low) / (high - low) * 100
	}
	location := "mid_range"
	switch {
	case pct >= 66:
		location = "upper_range"
	case pct <= 33:
		location = "lower_range"
	}
	return map[string]any{
		"location":     location,
		"position_pct": pct,
		"range_low":    low,
		"range_high":   high,
		"range_size":   high - low,
	}
}

func orderParams(symbol string, price, availableBalance float64, positionInfo map[string]any) map[string]any {
	quantity := 0.0
	if price > 0 && availableBalance > 0 {
		quantity = availableBalance * 0.1 / price
	}
	return map[string]any{
		"entry_price":       price,
		"stop_loss_price":   price * 0.99,
		"take_profit_price": price * 1.02,
		"quantity":          quantity,
		"position_1h":       positionInfo,
	}
}

func buildPositionsPayload(position *binance.Position) []map[string]any {
	if position == nil || position.PositionAmt == 0 {
		return []map[string]any{}
	}
	return []map[string]any{
		{
			"symbol":      position.Symbol,
			"quantity":    absFloat(position.PositionAmt),
			"entry_price": position.EntryPrice,
			"pnl":         position.UnrealizedProfit,
			"side":        positionSide(position),
			"leverage":    position.Leverage,
		},
	}
}

func buildTradeRecord(symbol string, decision domain.Decision, result execution.Result, currentPrice float64, position *binance.Position, account binance.FuturesAccount) map[string]any {
	side := strings.ToUpper(strings.TrimSpace(decision.Action))
	entryPrice := result.EntryPrice
	if entryPrice == 0 {
		if position != nil && position.EntryPrice > 0 {
			entryPrice = position.EntryPrice
		} else {
			entryPrice = currentPrice
		}
	}
	quantity := result.Quantity
	if quantity == 0 && position != nil {
		quantity = absFloat(position.PositionAmt)
	}
	pnl := 0.0
	if strings.HasPrefix(strings.ToLower(decision.Action), "close_") && position != nil && position.EntryPrice > 0 && quantity > 0 {
		if position.PositionAmt > 0 {
			pnl = (currentPrice - position.EntryPrice) * quantity
		} else if position.PositionAmt < 0 {
			pnl = (position.EntryPrice - currentPrice) * quantity
		}
	}
	return map[string]any{
		"recorded_at":  time.Now().Format("2006-01-02 15:04:05"),
		"timestamp":    time.Now().Format("2006-01-02 15:04:05"),
		"cycle":        0,
		"open_cycle":   0,
		"close_cycle":  ternary(strings.HasPrefix(strings.ToLower(decision.Action), "close_"), 1, 0),
		"symbol":       symbol,
		"side":         side,
		"action":       strings.ToUpper(decision.Action),
		"entry_price":  entryPrice,
		"exit_price":   ternary(strings.HasPrefix(strings.ToLower(decision.Action), "close_"), currentPrice, 0.0),
		"price":        currentPrice,
		"quantity":     quantity,
		"pnl":          pnl,
		"wallet_value": account.TotalWalletBalance,
	}
}

func (s *Server) executeVirtual(symbol string, decision domain.Decision) execution.Result {
	snap := s.state.Snapshot()
	virtualAccount, _ := snap["virtual_account"].(map[string]any)
	if len(virtualAccount) == 0 {
		virtualAccount = map[string]any{
			"initial_balance":         1000.0,
			"current_balance":         1000.0,
			"available_balance":       1000.0,
			"positions":               map[string]any{},
			"cumulative_realized_pnl": 0.0,
			"total_unrealized_pnl":    0.0,
		}
	}
	price := latestPriceFromState(snap, symbol)
	if price <= 0 {
		price = 1
	}
	s.state.UpdatePrice(symbol, price)
	action := strings.ToLower(strings.TrimSpace(decision.Action))
	positionsMap, _ := virtualAccount["positions"].(map[string]any)
	if positionsMap == nil {
		positionsMap = map[string]any{}
	}
	currentBalance := toFloat(virtualAccount["current_balance"])
	if currentBalance <= 0 {
		currentBalance = toFloat(virtualAccount["initial_balance"])
	}
	availableBalance := toFloat(virtualAccount["available_balance"])
	if availableBalance <= 0 {
		availableBalance = currentBalance
	}
	realized := toFloat(virtualAccount["cumulative_realized_pnl"])
	leverage := decision.Leverage
	if leverage <= 0 {
		leverage = float64(s.cfg.Trading.Leverage)
	}
	if leverage <= 0 {
		leverage = 5
	}
	result := execution.Result{
		Success:   false,
		Action:    action,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	switch action {
	case "", "wait", "hold":
		result.Success = true
		result.Message = "passive action (virtual)"
	case "open_long", "open_short":
		positionPct := decision.PositionSizePct
		if positionPct <= 0 {
			positionPct = 10
		}
		quantity := (availableBalance * (positionPct / 100.0) * leverage) / price
		quantity = roundQty(quantity)
		if quantity <= 0 {
			result.Message = "quantity is zero"
			break
		}
		marginUsed := price * quantity / leverage
		side := "LONG"
		if action == "open_short" {
			side = "SHORT"
		}
		positionsMap[symbol] = map[string]any{
			"quantity":       quantity,
			"entry_price":    price,
			"unrealized_pnl": 0.0,
			"side":           side,
			"leverage":       leverage,
		}
		availableBalance = maxNonZero(currentBalance-marginUsed, 0)
		result.Success = true
		result.Message = "position opened (virtual)"
		result.EntryPrice = price
		result.Quantity = quantity
		result.StopLoss = calcVirtualStopLoss(price, side, ternary(decision.StopLossPct > 0, decision.StopLossPct, 1.0))
		result.TakeProfit = calcVirtualTakeProfit(price, side, ternary(decision.TakeProfitPct > 0, decision.TakeProfitPct, 2.0))
	case "close_long", "close_short":
		rawPos, _ := positionsMap[symbol].(map[string]any)
		if rawPos == nil {
			result.Message = "no virtual position"
			break
		}
		side := strings.ToUpper(fmt.Sprint(rawPos["side"]))
		entry := toFloat(rawPos["entry_price"])
		quantity := toFloat(rawPos["quantity"])
		pnl := 0.0
		if side == "LONG" {
			pnl = (price - entry) * quantity
		} else {
			pnl = (entry - price) * quantity
		}
		currentBalance += pnl
		availableBalance = currentBalance
		realized += pnl
		delete(positionsMap, symbol)
		result.Success = true
		result.Message = "position closed (virtual)"
		result.EntryPrice = entry
		result.Quantity = quantity
	default:
		result.Message = "unknown action"
	}

	totalUnrealized := 0.0
	for sym, raw := range positionsMap {
		pos, _ := raw.(map[string]any)
		if pos == nil {
			continue
		}
		entry := toFloat(pos["entry_price"])
		quantity := toFloat(pos["quantity"])
		side := strings.ToUpper(fmt.Sprint(pos["side"]))
		mark := latestPriceFromState(snap, sym)
		if mark <= 0 {
			mark = entry
		}
		unrealized := 0.0
		if side == "LONG" {
			unrealized = (mark - entry) * quantity
		} else {
			unrealized = (entry - mark) * quantity
		}
		pos["unrealized_pnl"] = unrealized
		positionsMap[sym] = pos
		totalUnrealized += unrealized
	}

	virtualAccount["positions"] = positionsMap
	virtualAccount["current_balance"] = currentBalance
	virtualAccount["available_balance"] = availableBalance
	virtualAccount["cumulative_realized_pnl"] = realized
	virtualAccount["total_unrealized_pnl"] = totalUnrealized
	s.state.SetVirtualAccount(virtualAccount)
	s.state.UpdatePositions(buildVirtualPositionsPayload(positionsMap))
	s.state.AppendEquityPoint(currentBalance + totalUnrealized)
	s.state.AppendTrade(buildVirtualTradeRecord(symbol, decision, result, price))
	s.state.AddLog("execution: " + result.Message)
	return result
}

func latestPriceFromState(snapshot map[string]any, symbol string) float64 {
	currentPrice, _ := snapshot["current_price"].(map[string]float64)
	if currentPrice == nil {
		return 0
	}
	return currentPrice[symbol]
}

func buildVirtualPositionsPayload(positionsMap map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(positionsMap))
	for symbol, raw := range positionsMap {
		pos, _ := raw.(map[string]any)
		if pos == nil {
			continue
		}
		out = append(out, map[string]any{
			"symbol":      symbol,
			"quantity":    toFloat(pos["quantity"]),
			"entry_price": toFloat(pos["entry_price"]),
			"pnl":         toFloat(pos["unrealized_pnl"]),
			"side":        fmt.Sprint(pos["side"]),
			"leverage":    toFloat(pos["leverage"]),
		})
	}
	return out
}

func buildVirtualTradeRecord(symbol string, decision domain.Decision, result execution.Result, price float64) map[string]any {
	record := map[string]any{
		"recorded_at": time.Now().Format("2006-01-02 15:04:05"),
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"cycle":       0,
		"open_cycle":  0,
		"close_cycle": 0,
		"symbol":      symbol,
		"side":        strings.ToUpper(strings.TrimSpace(decision.Action)),
		"action":      strings.ToUpper(strings.TrimSpace(decision.Action)),
		"entry_price": ternary(result.EntryPrice > 0, result.EntryPrice, price),
		"price":       price,
		"quantity":    result.Quantity,
		"pnl":         0.0,
	}
	if strings.HasPrefix(strings.ToLower(decision.Action), "close_") {
		record["close_cycle"] = 1
		record["exit_price"] = price
	}
	return record
}

func addSemanticAgentMessages(st *state.SharedState, symbol string, semantic map[string]any) {
	mapping := []struct {
		key   string
		agent string
	}{
		{"trend_agent", "trend_agent"},
		{"setup_agent", "setup_agent"},
		{"trigger_agent", "trigger_agent"},
		{"multi_period_agent", "multi_period_agent"},
	}
	for _, item := range mapping {
		raw, ok := semantic[item.key].(map[string]any)
		if !ok {
			continue
		}
		stance := strings.TrimSpace(fmt.Sprint(raw["stance"]))
		summary := trimForPanel(fmt.Sprint(raw["summary"]))
		content := summary
		if stance != "" && stance != "<nil>" {
			content = fmt.Sprintf("%s | %s", compactStance(stance), summary)
		}
		st.AddAgentMessage(item.agent, content, "info", symbol)
	}
}

func trimForPanel(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " | "))
	if len(text) > 160 {
		return text[:160] + "..."
	}
	return text
}

func compactStance(v string) string {
	v = strings.TrimSpace(strings.ToUpper(v))
	switch {
	case strings.Contains(v, "BULL"):
		return "看多"
	case strings.Contains(v, "BEAR"):
		return "看空"
	case strings.Contains(v, "NEUTRAL"):
		return "中性"
	default:
		return v
	}
}

func roundQty(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return float64(int(v*1000)) / 1000
}

func calcVirtualStopLoss(entry float64, side string, pct float64) float64 {
	if strings.EqualFold(side, "LONG") {
		return entry * (1 - pct/100.0)
	}
	return entry * (1 + pct/100.0)
}

func calcVirtualTakeProfit(entry float64, side string, pct float64) float64 {
	if strings.EqualFold(side, "LONG") {
		return entry * (1 + pct/100.0)
	}
	return entry * (1 - pct/100.0)
}

func trendAlignment(scores ...float64) bool {
	hasPos := false
	hasNeg := false
	for _, score := range scores {
		if score > 5 {
			hasPos = true
		}
		if score < -5 {
			hasNeg = true
		}
	}
	return !(hasPos && hasNeg)
}

func stanceFromScore(v any) string {
	score, _ := v.(float64)
	switch {
	case score >= 20:
		return "bullish"
	case score <= -20:
		return "bearish"
	default:
		return "neutral"
	}
}

func stanceStrength(stance string) string {
	switch stance {
	case "bullish", "bearish":
		return "strong"
	default:
		return "balanced"
	}
}

func trimAlignment(status string) string {
	switch {
	case containsAny(status, "bull"):
		return "bullish"
	case containsAny(status, "bear"):
		return "bearish"
	default:
		return "mixed"
	}
}

func bandPosition(price, ema20, ema60 float64) string {
	mid := (ema20 + ema60) / 2
	upper := maxNonZero(ema20, ema60)
	lower := minNonZero(ema20, ema60)
	switch {
	case price >= upper:
		return "upper"
	case price <= lower:
		return "lower"
	case price >= mid:
		return "middle"
	default:
		return "middle"
	}
}

func firstWarningOrDefault(riskResult risk.Result, fallback string) string {
	if riskResult.BlockedReason != "" {
		return riskResult.BlockedReason
	}
	if len(riskResult.Warnings) > 0 {
		return riskResult.Warnings[0]
	}
	return fallback
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func semanticAnalysesFromRequest(agentData map[string]any, regime map[string]any) map[string]any {
	out := map[string]any{}
	for key, alias := range map[string]string{
		"trend_agent":        "trend",
		"setup_agent":        "setup",
		"trigger_agent":      "trigger",
		"multi_period_agent": "multi_period",
	} {
		raw, ok := agentData[key].(map[string]any)
		if !ok {
			continue
		}
		entry := cloneMap(raw)
		if _, exists := entry["metadata"]; !exists {
			entry["metadata"] = map[string]any{}
		}
		meta, _ := entry["metadata"].(map[string]any)
		if alias == "trend" && meta["adx"] == nil {
			meta["adx"] = regime["adx"]
		}
		if alias == "trend" && meta["strength"] == nil {
			meta["strength"] = stanceStrength(fmt.Sprint(entry["stance"]))
		}
		if alias == "trigger" && meta["pattern"] == nil {
			meta["pattern"] = deriveTriggerPattern(fmt.Sprint(entry["summary"]))
		}
		if alias == "trigger" && meta["rvol"] == nil {
			meta["rvol"] = 1.0
		}
		entry["metadata"] = meta
		out[alias] = entry
	}
	return out
}

func predictProbability(predict map[string]any) float64 {
	for _, key := range []string{"p_up", "probability", "prophet", "prophet_probability"} {
		if v, ok := predict[key]; ok {
			p := toFloat(v)
			if p > 1 {
				p = p / 100
			}
			if p > 0 {
				return p
			}
		}
	}
	return 0.5
}

func summaryFromAgent(agentData map[string]any, key string) string {
	raw, _ := agentData[key].(map[string]any)
	if raw == nil {
		return ""
	}
	return fmt.Sprint(raw["summary"])
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	case map[string]any:
		if score, ok := x["score"]; ok {
			return toFloat(score)
		}
	}
	return 0
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func containsAny(text string, patterns ...string) bool {
	for _, p := range patterns {
		if p != "" && strings.Contains(strings.ToLower(text), strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func minNonZero(a, b float64) float64 {
	if a == 0 {
		return b
	}
	if b == 0 || a < b {
		return a
	}
	return b
}

func maxNonZero(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func defaultAgentConfig() map[string]bool {
	return map[string]bool{
		"predict_agent":              true,
		"ai_prediction_filter_agent": true,
		"regime_detector_agent":      true,
		"position_analyzer_agent":    false,
		"trigger_detector_agent":     true,
		"trend_agent_llm":            false,
		"setup_agent_llm":            false,
		"trigger_agent_llm":          false,
		"trend_agent_local":          true,
		"setup_agent_local":          true,
		"trigger_agent_local":        true,
		"reflection_agent_llm":       false,
		"reflection_agent_local":     true,
		"symbol_selector_agent":      true,
	}
}

func defaultAgentSettings() map[string]any {
	return map[string]any{
		"symbol_selector":  map[string]any{"params": map[string]any{}, "system_prompt": ""},
		"trend_agent":      map[string]any{"params": map[string]any{"temperature": 0.3, "max_tokens": 800}, "system_prompt": ""},
		"setup_agent":      map[string]any{"params": map[string]any{"temperature": 0.3, "max_tokens": 800}, "system_prompt": ""},
		"trigger_agent":    map[string]any{"params": map[string]any{"temperature": 0.3, "max_tokens": 800}, "system_prompt": ""},
		"multi_period":     map[string]any{"params": map[string]any{}, "system_prompt": ""},
		"reflection_agent": map[string]any{"params": map[string]any{"temperature": 0.3, "max_tokens": 1200}, "system_prompt": ""},
		"risk_audit":       map[string]any{"params": map[string]any{"max_leverage": 10, "max_position_pct": 35}, "system_prompt": ""},
		"decision_core":    map[string]any{"params": map[string]any{"temperature": 0.3, "max_tokens": 2000}, "system_prompt": ""},
	}
}
