package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"llmtradebot/internal/config"
	"llmtradebot/internal/domain"
)

type Engine struct {
	cfg    config.LLMConfig
	client *http.Client
}

func New(cfg config.LLMConfig) *Engine {
	return &Engine{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (e *Engine) Ready() bool {
	return e.cfg.APIKey != "" && e.cfg.BaseURL != "" && e.cfg.Model != ""
}

func (e *Engine) GetBullPerspective(ctx context.Context, marketContext string) (domain.Perspective, error) {
	decision, raw, err := e.complete(ctx, bullishPrompt, marketContext, 500)
	if err != nil {
		return domain.Perspective{}, err
	}
	return domain.Perspective{
		Stance:     getString(decision, "stance", "UNKNOWN"),
		Reasons:    getString(decision, "bullish_reasons", "N/A"),
		Confidence: int(getFloat(decision, "bull_confidence")),
	}, rawErrWrap(raw, nil)
}

func (e *Engine) GetBearPerspective(ctx context.Context, marketContext string) (domain.Perspective, error) {
	decision, raw, err := e.complete(ctx, bearishPrompt, marketContext, 500)
	if err != nil {
		return domain.Perspective{}, err
	}
	return domain.Perspective{
		Stance:     getString(decision, "stance", "UNKNOWN"),
		Reasons:    getString(decision, "bearish_reasons", "N/A"),
		Confidence: int(getFloat(decision, "bear_confidence")),
	}, rawErrWrap(raw, nil)
}

func (e *Engine) MakeDecision(ctx context.Context, marketContext, reflection string, bull, bear domain.Perspective) (domain.Decision, error) {
	userPrompt := buildDecisionPrompt(marketContext, reflection, bull, bear)
	parsed, raw, err := e.complete(ctx, defaultSystemPrompt, userPrompt, e.cfg.MaxTokens)
	if err != nil {
		return fallbackDecision(err), err
	}

	return domain.Decision{
		Action:          strings.ToLower(getString(parsed, "action", "wait")),
		Confidence:      getFloat(parsed, "confidence"),
		Leverage:        getFloat(parsed, "leverage"),
		PositionSizePct: getFloat(parsed, "position_size_pct"),
		StopLossPct:     getFloat(parsed, "stop_loss_pct"),
		TakeProfitPct:   getFloat(parsed, "take_profit_pct"),
		Reasoning:       getString(parsed, "reasoning", "LLM decision generated"),
		BullPerspective: bull,
		BearPerspective: bear,
		RawResponse:     raw,
	}, nil
}

func (e *Engine) complete(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (map[string]any, string, error) {
	if !e.Ready() {
		return nil, "", fmt.Errorf("llm engine not ready")
	}

	payload := map[string]any{
		"model": e.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": e.cfg.Temperature,
		"max_tokens":  maxTokens,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(e.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("llm request failed: %s: %s", resp.Status, string(rawBody))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rawBody, &apiResp); err != nil {
		return nil, "", err
	}
	if len(apiResp.Choices) == 0 {
		return nil, "", fmt.Errorf("llm response missing choices")
	}

	content := apiResp.Choices[0].Message.Content
	parsed, err := extractJSON(content)
	return parsed, content, err
}

func extractJSON(text string) (map[string]any, error) {
	re := regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\}|\\[.*?\\])\\s*```")
	if m := re.FindStringSubmatch(text); len(m) == 2 {
		return decodeMap(m[1])
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return decodeMap(text[start : end+1])
	}
	return nil, fmt.Errorf("no json object found in llm response")
}

func decodeMap(raw string) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		return obj, nil
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(raw), &arr); err == nil && len(arr) > 0 {
		return arr[0], nil
	}
	return nil, fmt.Errorf("json decode failed")
}

func buildDecisionPrompt(marketContext, reflection string, bull, bear domain.Perspective) string {
	var b strings.Builder
	b.WriteString("# Market Data Input\n\n")
	b.WriteString(marketContext)
	b.WriteString("\n\n## Bull Perspective\n")
	b.WriteString(fmt.Sprintf("Stance: %s\nReasons: %s\nConfidence: %d\n", bull.Stance, bull.Reasons, bull.Confidence))
	b.WriteString("\n## Bear Perspective\n")
	b.WriteString(fmt.Sprintf("Stance: %s\nReasons: %s\nConfidence: %d\n", bear.Stance, bear.Reasons, bear.Confidence))
	if strings.TrimSpace(reflection) != "" {
		b.WriteString("\n## Reflection\n")
		b.WriteString(reflection)
	}
	b.WriteString("\n\nReturn JSON only.")
	return b.String()
}

func fallbackDecision(err error) domain.Decision {
	reason := "LLM decision failed, using conservative fallback strategy"
	if err != nil {
		reason = reason + ": " + err.Error()
	}
	return domain.Decision{
		Action:          "wait",
		Confidence:      0,
		PositionSizePct: 0,
		Reasoning:       reason,
	}
}

func getString(m map[string]any, key, fallback string) string {
	v, ok := m[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func getFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

func rawErrWrap(_ string, err error) error {
	return err
}

const bullishPrompt = `You are a BULLISH market analyst. Output JSON only:
{"stance":"STRONGLY_BULLISH","bullish_reasons":"...","bull_confidence":75}`

const bearishPrompt = `You are a BEARISH market analyst. Output JSON only:
{"stance":"STRONGLY_BEARISH","bearish_reasons":"...","bear_confidence":75}`

const defaultSystemPrompt = `You are the final trading decision engine.
Return JSON only with:
{
  "action": "wait|open_long|open_short|close_long|close_short|hold",
  "confidence": 0,
  "position_size_pct": 0,
  "reasoning": "brief explanation"
}`
