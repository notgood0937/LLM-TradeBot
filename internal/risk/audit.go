package risk

import (
	"fmt"
	"math"
	"strings"

	"llmtradebot/internal/domain"
)

type Auditor struct {
	MaxRiskPerTradePct float64
	MaxLeverage        float64
	MaxPositionPct     float64
	MinStopLossPct     float64
	MaxStopLossPct     float64
}

type Result struct {
	Passed        bool     `json:"passed"`
	RiskLevel     string   `json:"risk_level"`
	BlockedReason string   `json:"blocked_reason,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type CheckInput struct {
	Decision       domain.Decision `json:"decision"`
	AccountBalance float64         `json:"account_balance"`
	CurrentPrice   float64         `json:"current_price"`
	HasPosition    bool            `json:"has_position"`
	PositionSide   string          `json:"position_side"`
}

func New() *Auditor {
	return &Auditor{
		MaxRiskPerTradePct: 1.5,
		MaxLeverage:        10,
		MaxPositionPct:     35,
		MinStopLossPct:     0.2,
		MaxStopLossPct:     2.5,
	}
}

func (a *Auditor) Audit(in CheckInput) Result {
	action := strings.ToLower(strings.TrimSpace(in.Decision.Action))
	if action == "" || action == "wait" || action == "hold" {
		return Result{Passed: true, RiskLevel: "safe", Warnings: []string{"passive decision"}}
	}

	if strings.HasPrefix(action, "open_") && in.AccountBalance <= 0 {
		return Result{Passed: false, RiskLevel: "fatal", BlockedReason: "insufficient_balance"}
	}

	if strings.HasPrefix(action, "open_") && in.Decision.Leverage > 0 && in.Decision.Leverage > a.MaxLeverage {
		return Result{Passed: false, RiskLevel: "danger", BlockedReason: "over_leverage"}
	}

	if strings.HasPrefix(action, "open_") && in.HasPosition {
		if action == "open_long" && strings.EqualFold(in.PositionSide, "SHORT") {
			return Result{Passed: false, RiskLevel: "danger", BlockedReason: "reverse_position_block"}
		}
		if action == "open_short" && strings.EqualFold(in.PositionSide, "LONG") {
			return Result{Passed: false, RiskLevel: "danger", BlockedReason: "reverse_position_block"}
		}
	}

	if in.Decision.PositionSizePct > a.MaxPositionPct {
		return Result{Passed: false, RiskLevel: "danger", BlockedReason: fmt.Sprintf("position_size_pct_exceeded: %.1f", in.Decision.PositionSizePct)}
	}

	if strings.HasPrefix(action, "open_") {
		if in.Decision.StopLossPct > 0 {
			if in.Decision.StopLossPct < a.MinStopLossPct {
				return Result{Passed: false, RiskLevel: "warning", BlockedReason: "stop_loss_too_tight"}
			}
			if in.Decision.StopLossPct > a.MaxStopLossPct {
				return Result{Passed: false, RiskLevel: "danger", BlockedReason: "stop_loss_too_wide"}
			}
		}
		if in.Decision.StopLossPct > 0 && in.Decision.TakeProfitPct > 0 {
			rr := in.Decision.TakeProfitPct / in.Decision.StopLossPct
			if rr < 2.0 {
				return Result{Passed: false, RiskLevel: "warning", BlockedReason: fmt.Sprintf("risk_reward_too_low: %.2f", rr)}
			}
		}
		riskBudget := (in.Decision.PositionSizePct / 100.0) * maxFloat(in.Decision.StopLossPct, 1.0)
		if riskBudget > a.MaxRiskPerTradePct {
			return Result{Passed: false, RiskLevel: "danger", BlockedReason: "risk_per_trade_exceeded"}
		}
	}

	warnings := []string{}
	if in.Decision.Confidence < 60 && strings.HasPrefix(action, "open_") {
		warnings = append(warnings, "low confidence open action")
	}
	if math.Abs(in.Decision.PositionSizePct) == 0 && strings.HasPrefix(action, "open_") {
		warnings = append(warnings, "open action with zero position size")
	}
	return Result{Passed: true, RiskLevel: "safe", Warnings: warnings}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
