package execution

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"llmtradebot/internal/binance"
	"llmtradebot/internal/config"
	"llmtradebot/internal/domain"
)

type Engine struct {
	client *binance.Client
	cfg    config.Config
}

type Result struct {
	Success    bool            `json:"success"`
	Action     string          `json:"action"`
	Timestamp  string          `json:"timestamp"`
	Message    string          `json:"message"`
	Orders     []binance.Order `json:"orders,omitempty"`
	EntryPrice float64         `json:"entry_price,omitempty"`
	Quantity   float64         `json:"quantity,omitempty"`
	StopLoss   float64         `json:"stop_loss,omitempty"`
	TakeProfit float64         `json:"take_profit,omitempty"`
}

func New(client *binance.Client, cfg config.Config) *Engine {
	return &Engine{client: client, cfg: cfg}
}

func (e *Engine) ExecuteDecision(ctx context.Context, decision domain.Decision, account binance.FuturesAccount, position *binance.Position, currentPrice float64, symbol string) Result {
	action := strings.ToLower(strings.TrimSpace(decision.Action))
	result := Result{
		Success:   false,
		Action:    action,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	switch action {
	case "", "wait", "hold":
		result.Success = true
		result.Message = "passive action"
		return result
	case "open_long":
		return e.openPosition(ctx, symbol, "BUY", "LONG", decision, account, currentPrice)
	case "open_short":
		return e.openPosition(ctx, symbol, "SELL", "SHORT", decision, account, currentPrice)
	case "close_long", "close_short":
		return e.closePosition(ctx, symbol, action, position)
	default:
		result.Message = "unknown action"
		return result
	}
}

func (e *Engine) openPosition(ctx context.Context, symbol, side, positionSide string, decision domain.Decision, account binance.FuturesAccount, currentPrice float64) Result {
	res := Result{Action: strings.ToLower(side), Timestamp: time.Now().Format(time.RFC3339)}
	leverage := e.cfg.Trading.Leverage
	if leverage <= 0 {
		leverage = 5
	}
	if err := e.client.SetLeverage(ctx, symbol, leverage); err != nil {
		res.Message = fmt.Sprintf("set leverage failed: %v", err)
		return res
	}
	quantity := calculatePositionSize(account.AvailableBalance, decision.PositionSizePct, float64(leverage), currentPrice)
	if quantity <= 0 {
		res.Message = "quantity is zero"
		return res
	}
	order, err := e.client.PlaceMarketOrder(ctx, symbol, side, quantity, false, positionSide)
	if err != nil {
		res.Message = err.Error()
		return res
	}
	entryPrice := order.AvgPrice
	if entryPrice == 0 {
		entryPrice = currentPrice
	}
	stopLoss := calcStopLoss(entryPrice, positionSide, 1.0)
	takeProfit := calcTakeProfit(entryPrice, positionSide, 2.0)
	auxOrders, err := e.client.SetStopLossTakeProfit(ctx, symbol, &stopLoss, &takeProfit, positionSide)
	if err != nil {
		res.Message = "opened but failed to set sl/tp: " + err.Error()
		res.Success = true
		res.Orders = []binance.Order{order}
		res.EntryPrice = entryPrice
		res.Quantity = quantity
		return res
	}
	res.Success = true
	res.Message = "position opened"
	res.Orders = append([]binance.Order{order}, auxOrders...)
	res.EntryPrice = entryPrice
	res.Quantity = quantity
	res.StopLoss = stopLoss
	res.TakeProfit = takeProfit
	return res
}

func (e *Engine) closePosition(ctx context.Context, symbol, action string, position *binance.Position) Result {
	res := Result{Action: action, Timestamp: time.Now().Format(time.RFC3339)}
	if position == nil || position.PositionAmt == 0 {
		res.Message = "no position"
		return res
	}
	if action == "close_long" && position.PositionAmt < 0 {
		res.Message = "position side mismatch"
		return res
	}
	if action == "close_short" && position.PositionAmt > 0 {
		res.Message = "position side mismatch"
		return res
	}
	side := "SELL"
	positionSide := "LONG"
	qty := math.Abs(position.PositionAmt)
	if position.PositionAmt < 0 {
		side = "BUY"
		positionSide = "SHORT"
	}
	_ = e.client.CancelAllOrders(ctx, symbol)
	order, err := e.client.PlaceMarketOrder(ctx, symbol, side, qty, true, positionSide)
	if err != nil {
		res.Message = err.Error()
		return res
	}
	res.Success = true
	res.Message = "position closed"
	res.Orders = []binance.Order{order}
	return res
}

func calculatePositionSize(balance, positionPct, leverage, currentPrice float64) float64 {
	if balance <= 0 || currentPrice <= 0 || positionPct <= 0 || leverage <= 0 {
		return 0
	}
	notional := balance * (positionPct / 100.0) * leverage
	qty := notional / currentPrice
	if qty < 0 {
		return 0
	}
	return math.Floor(qty*1000) / 1000
}

func calcStopLoss(entry float64, side string, pct float64) float64 {
	if strings.EqualFold(side, "LONG") {
		return entry * (1 - pct/100.0)
	}
	return entry * (1 + pct/100.0)
}

func calcTakeProfit(entry float64, side string, pct float64) float64 {
	if strings.EqualFold(side, "LONG") {
		return entry * (1 + pct/100.0)
	}
	return entry * (1 - pct/100.0)
}
