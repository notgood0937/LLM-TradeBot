package app

import (
	"context"
	"fmt"
	"log"
	"strings"

	"llmtradebot/internal/config"
	"llmtradebot/internal/domain"
	"llmtradebot/internal/llm"
	"llmtradebot/internal/telegram"
)

type DecisionRequest = domain.DecisionRequest

type DecisionService struct {
	engine   *llm.Engine
	notifier *telegram.Notifier
}

func NewDecisionService(cfg config.Config) *DecisionService {
	return &DecisionService{
		engine:   llm.New(cfg.LLM),
		notifier: telegram.New(cfg.Telegram),
	}
}

func (s *DecisionService) Run(ctx context.Context, req DecisionRequest) (domain.Decision, string, error) {
	marketContext := buildMarketContext(req)
	logAIBlock("AI 上下文", []string{
		fmt.Sprintf("币种: %s", req.Symbol),
		fmt.Sprintf("价格: %.4f", req.CurrentPrice),
		fmt.Sprintf("Quant: %d | Predict: %d | Regime: %d | Position: %d | Agents: %d",
			len(req.QuantAnalysis),
			len(req.PredictResult),
			len(req.RegimeInfo),
			len(req.CurrentPosition),
			len(req.SelectedAgentData),
		),
	}, ansiBlue)

	bull, bullErr := s.engine.GetBullPerspective(ctx, marketContext)
	if bullErr != nil {
		bull = domain.Perspective{Stance: "NEUTRAL", Reasons: bullErr.Error(), Confidence: 50}
	}
	logAIBlock("多头分析", []string{
		fmt.Sprintf("币种: %s", req.Symbol),
		fmt.Sprintf("立场: %s", compactStance(bull.Stance)),
		fmt.Sprintf("置信度: %d%%", bull.Confidence),
		fmt.Sprintf("理由: %s", trimLogText(bull.Reasons)),
		errLine(bullErr),
	}, ansiGreen)

	bear, bearErr := s.engine.GetBearPerspective(ctx, marketContext)
	if bearErr != nil {
		bear = domain.Perspective{Stance: "NEUTRAL", Reasons: bearErr.Error(), Confidence: 50}
	}
	logAIBlock("空头分析", []string{
		fmt.Sprintf("币种: %s", req.Symbol),
		fmt.Sprintf("立场: %s", compactStance(bear.Stance)),
		fmt.Sprintf("置信度: %d%%", bear.Confidence),
		fmt.Sprintf("理由: %s", trimLogText(bear.Reasons)),
		errLine(bearErr),
	}, ansiRed)

	decision, err := s.engine.MakeDecision(ctx, marketContext, req.ReflectionText, bull, bear)
	label, icon := decisionLabel(decision.Action)
	logAIBlock("最终决策", []string{
		fmt.Sprintf("币种: %s", req.Symbol),
		fmt.Sprintf("动作: %s %s", icon, label),
		fmt.Sprintf("置信度: %.1f%%", decision.Confidence),
		fmt.Sprintf("理由: %s", trimLogText(decision.Reasoning)),
		errLine(err),
	}, ansiMagenta)
	s.notify(compactCycleMessage(req.Symbol, bull, bear, decision))
	return decision, marketContext, err
}

func buildMarketContext(req DecisionRequest) string {
	var b strings.Builder
	b.WriteString("## Snapshot\n")
	b.WriteString(fmt.Sprintf("- Symbol: %s\n", req.Symbol))
	b.WriteString(fmt.Sprintf("- Current Price: %.4f\n", req.CurrentPrice))

	writeSection(&b, "Quant Analysis", req.QuantAnalysis)
	writeSection(&b, "Predict Result", req.PredictResult)
	writeSection(&b, "Regime Info", req.RegimeInfo)
	writeSection(&b, "Current Position", req.CurrentPosition)
	writeSection(&b, "Selected Agent Outputs", req.SelectedAgentData)
	return b.String()
}

func writeSection(b *strings.Builder, title string, value map[string]any) {
	if len(value) == 0 {
		return
	}
	b.WriteString("\n## " + title + "\n")
	for k, v := range value {
		b.WriteString(fmt.Sprintf("- %s: %v\n", k, v))
	}
}

func (s *DecisionService) notify(message string) {
	if s.notifier == nil || !s.notifier.Ready() {
		log.Printf("telegram: skipped ready=false")
		return
	}
	s.notifier.SendAsync(message)
}

func trimLogText(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " | "))
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

func compactCycleMessage(symbol string, bull, bear domain.Perspective, decision domain.Decision) string {
	actionLabel, actionIcon := decisionLabel(decision.Action)
	reason := strings.TrimSpace(strings.ReplaceAll(decision.Reasoning, "\n", " "))
	return strings.Join([]string{
		fmt.Sprintf("<b>%s %s</b>", actionIcon, actionLabel),
		fmt.Sprintf("币种：<code>%s</code>", symbol),
		fmt.Sprintf("置信度：<b>%.0f%%</b>", decision.Confidence),
		fmt.Sprintf("多头：%s <b>%d%%</b>", compactStance(bull.Stance), bull.Confidence),
		fmt.Sprintf("空头：%s <b>%d%%</b>", compactStance(bear.Stance), bear.Confidence),
		fmt.Sprintf("理由：%s", escapeTelegram(reason)),
	}, "\n")
}

func decisionLabel(action string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long":
		return "开多", "🟢"
	case "open_short":
		return "开空", "🔴"
	case "close_long":
		return "平多", "⚪"
	case "close_short":
		return "平空", "⚪"
	case "hold":
		return "持有", "🟡"
	default:
		return "观望", "🔵"
	}
}

func escapeTelegram(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(text)
}

const (
	ansiReset   = "\033[0m"
	ansiBlue    = "\033[38;5;45m"
	ansiGreen   = "\033[38;5;42m"
	ansiRed     = "\033[38;5;203m"
	ansiMagenta = "\033[38;5;177m"
)

func logAIBlock(title string, lines []string, color string) {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	border := color + "------------------------------------------------------------" + ansiReset
	log.Print(border)
	log.Printf("%s%s%s", color, title, ansiReset)
	for _, line := range filtered {
		log.Printf("%s%s%s", color, line, ansiReset)
	}
	log.Print(border)
}

func errLine(err error) string {
	if err == nil {
		return ""
	}
	return "异常: " + trimLogText(err.Error())
}
