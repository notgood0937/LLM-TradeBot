package state

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type SharedState struct {
	mu sync.RWMutex

	IsRunning       bool               `json:"is_running"`
	ExecutionMode   string             `json:"execution_mode"`
	IsTestMode      bool               `json:"is_test_mode"`
	StartTime       string             `json:"start_time"`
	LastUpdate      string             `json:"last_update"`
	CycleCounter    int                `json:"cycle_counter"`
	CurrentSymbol   string             `json:"current_symbol"`
	Symbols         []string           `json:"symbols"`
	CurrentPrice    map[string]float64 `json:"current_price"`
	AccountOverview map[string]float64 `json:"account_overview"`
	LatestDecision  map[string]any     `json:"latest_decision"`
	DecisionHistory []map[string]any   `json:"decision_history"`
	Positions       []map[string]any   `json:"positions"`
	TradeHistory    []map[string]any   `json:"trade_history"`
	ChartData       map[string]any     `json:"chart_data"`
	VirtualAccount  map[string]any     `json:"virtual_account"`
	RecentLogs      []string           `json:"recent_logs"`
	AgentMessages   []map[string]any   `json:"agent_messages"`
	SymbolSelector  map[string]any     `json:"symbol_selector"`
}

func New(symbols []string) *SharedState {
	now := time.Now().Format("2006-01-02 15:04:05")
	return &SharedState{
		IsRunning:     false,
		ExecutionMode: "Stopped",
		StartTime:     now,
		LastUpdate:    now,
		Symbols:       symbols,
		CurrentPrice:  map[string]float64{},
		AccountOverview: map[string]float64{
			"total_equity":      0,
			"available_balance": 0,
			"wallet_balance":    0,
			"total_pnl":         0,
		},
		LatestDecision:  map[string]any{},
		DecisionHistory: []map[string]any{},
		Positions:       []map[string]any{},
		TradeHistory:    []map[string]any{},
		ChartData: map[string]any{
			"equity_curve": []map[string]any{},
		},
		VirtualAccount: map[string]any{
			"initial_balance":         1000.0,
			"current_balance":         1000.0,
			"available_balance":       1000.0,
			"positions":               map[string]any{},
			"cumulative_realized_pnl": 0.0,
			"total_unrealized_pnl":    0.0,
		},
		RecentLogs:     []string{},
		AgentMessages:  []map[string]any{},
		SymbolSelector: map[string]any{},
	}
}

func (s *SharedState) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"is_running":       s.IsRunning,
		"execution_mode":   s.ExecutionMode,
		"is_test_mode":     s.IsTestMode,
		"start_time":       s.StartTime,
		"last_update":      s.LastUpdate,
		"cycle_counter":    s.CycleCounter,
		"current_symbol":   s.CurrentSymbol,
		"symbols":          s.Symbols,
		"current_price":    s.CurrentPrice,
		"account_overview": s.AccountOverview,
		"latest_decision":  s.LatestDecision,
		"decision_history": s.DecisionHistory,
		"positions":        s.Positions,
		"trade_history":    s.TradeHistory,
		"chart_data":       s.ChartData,
		"virtual_account":  s.VirtualAccount,
		"recent_logs":      s.RecentLogs,
		"agent_messages":   s.AgentMessages,
		"symbol_selector":  s.SymbolSelector,
	}
}

func (s *SharedState) AddLog(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
	s.RecentLogs = append(s.RecentLogs, s.LastUpdate+" "+message)
	if len(s.RecentLogs) > 500 {
		s.RecentLogs = s.RecentLogs[len(s.RecentLogs)-500:]
	}
}

func (s *SharedState) AddAgentMessage(agent, content, level, symbol string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := map[string]any{
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"agent":     agent,
		"content":   content,
		"level":     level,
		"symbol":    symbol,
		"cycle":     s.CycleCounter,
	}
	s.AgentMessages = append(s.AgentMessages, msg)
	log.Printf("agent_message: agent=%s level=%s symbol=%s cycle=%d content=%s", agent, level, symbol, s.CycleCounter, content)
	if len(s.AgentMessages) > 100 {
		s.AgentMessages = s.AgentMessages[len(s.AgentMessages)-100:]
	}
}

func (s *SharedState) UpdateDecision(symbol string, decision map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LatestDecision = decision
	if symbol != "" {
		decision["symbol"] = symbol
		s.CurrentSymbol = symbol
	}
	s.CycleCounter++
	s.DecisionHistory = append([]map[string]any{decision}, s.DecisionHistory...)
	if len(s.DecisionHistory) > 100 {
		s.DecisionHistory = s.DecisionHistory[:100]
	}
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
}

func (s *SharedState) UpdateAccount(totalEquity, available, wallet, pnl float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AccountOverview["total_equity"] = totalEquity
	s.AccountOverview["available_balance"] = available
	s.AccountOverview["wallet_balance"] = wallet
	s.AccountOverview["total_pnl"] = pnl
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
	s.appendEquityPointLocked(totalEquity)
}

func (s *SharedState) UpdatePrice(symbol string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentPrice[symbol] = price
	s.CurrentSymbol = symbol
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
}

func (s *SharedState) UpdateSelector(result map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SymbolSelector = result
}

func (s *SharedState) UpdatePositions(positions []map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Positions = positions
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
}

func (s *SharedState) AppendTrade(trade map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TradeHistory = append([]map[string]any{trade}, s.TradeHistory...)
	if len(s.TradeHistory) > 200 {
		s.TradeHistory = s.TradeHistory[:200]
	}
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
}

func (s *SharedState) SetVirtualAccount(account map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VirtualAccount = account
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
}

func (s *SharedState) AppendEquityPoint(value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendEquityPointLocked(value)
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
}

func (s *SharedState) appendEquityPointLocked(totalEquity float64) {
	points, _ := s.ChartData["equity_curve"].([]map[string]any)
	points = append(points, map[string]any{
		"time":  time.Now().Format("2006-01-02 15:04:05"),
		"value": totalEquity,
	})
	if len(points) > 500 {
		points = points[len(points)-500:]
	}
	s.ChartData["equity_curve"] = points
}

func (s *SharedState) SetMode(running bool, mode string, test bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsRunning = running
	s.ExecutionMode = mode
	s.IsTestMode = test
	s.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
}

func (s *SharedState) Persist(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *SharedState) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var tmp SharedState
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp.mu = sync.RWMutex{}
	*s = tmp
	return nil
}
