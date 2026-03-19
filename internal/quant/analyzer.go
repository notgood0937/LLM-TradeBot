package quant

import (
	"math"
	"strconv"
	"strings"

	"llmtradebot/internal/binance"
	"llmtradebot/internal/config"
)

type Analyzer struct {
	cfg *config.StrategyConfig
}

type Analysis struct {
	Trend         map[string]any `json:"trend"`
	Oscillator    map[string]any `json:"oscillator"`
	Sentiment     map[string]any `json:"sentiment"`
	Comprehensive map[string]any `json:"comprehensive"`
	Regime        map[string]any `json:"regime"`
	Semantic      map[string]any `json:"semantic"`
}

func New(cfg *config.StrategyConfig) *Analyzer {
	return &Analyzer{cfg: cfg}
}

func (a *Analyzer) Analyze(tf1h, tf15m, tf5m []binance.Candle) Analysis {
	trend1h, trend1hDetails := a.trendScore(tf1h)
	trend15m, trend15mDetails := a.trendScore(tf15m)
	trend5m, trend5mDetails := a.trendScore(tf5m)

	osc1h, osc1hDetails := a.oscillatorScore(tf1h)
	osc15m, osc15mDetails := a.oscillatorScore(tf15m)
	osc5m, osc5mDetails := a.oscillatorScore(tf5m)

	sentimentScore := 0.0
	if len(tf5m) >= 20 {
		volNow := avgVolume(tf5m[max(0, len(tf5m)-5):])
		volBase := avgVolume(tf5m[max(0, len(tf5m)-20):max(0, len(tf5m)-5)])
		if volBase > 0 {
			rvol := volNow / volBase
			if rvol > 1.5 {
				sentimentScore = 15
			} else if rvol < 0.7 {
				sentimentScore = -10
			}
		}
	}

	totalTrend := trend1h + trend15m + trend5m
	totalOsc := osc1h + osc15m + osc5m
	total := totalTrend*0.55 + totalOsc*0.25 + sentimentScore*0.20
	regime := detectRegime(tf1h, totalTrend, totalOsc)

	return Analysis{
		Trend: map[string]any{
			"trend_1h_score":    trend1h,
			"trend_15m_score":   trend15m,
			"trend_5m_score":    trend5m,
			"total_trend_score": totalTrend,
			"details": map[string]any{
				"1h":  trend1hDetails,
				"15m": trend15mDetails,
				"5m":  trend5mDetails,
			},
		},
		Oscillator: map[string]any{
			"osc_1h_score":    osc1h,
			"osc_15m_score":   osc15m,
			"osc_5m_score":    osc5m,
			"total_osc_score": totalOsc,
			"oscillator_1h":   map[string]any{"details": osc1hDetails},
			"oscillator_15m":  map[string]any{"details": osc15mDetails},
			"oscillator_5m":   map[string]any{"details": osc5mDetails},
		},
		Sentiment: map[string]any{
			"total_sentiment_score": sentimentScore,
		},
		Comprehensive: map[string]any{
			"score": total,
		},
		Regime: regime,
		Semantic: map[string]any{
			"trend_agent": map[string]any{
				"stance":  trendStance(totalTrend),
				"summary": buildTrendSummary(totalTrend, trend1h, trend15m, trend5m),
			},
			"setup_agent": map[string]any{
				"stance":  oscillatorStance(totalOsc),
				"summary": buildSetupSummary(totalOsc, osc15mDetails),
			},
			"trigger_agent": map[string]any{
				"stance":  triggerStance(tf5m),
				"summary": buildTriggerSummary(tf5m),
			},
			"multi_period_agent": map[string]any{
				"summary": buildMultiPeriodSummary(totalTrend, totalOsc, regime),
			},
		},
	}
}

func (a *Analyzer) trendScore(c []binance.Candle) (float64, map[string]any) {
	if len(c) < a.cfg.EmaLongPeriod {
		return 0, map[string]any{}
	}
	closeSeries := closes(c)
	ema20 := ema(closeSeries, a.cfg.EmaShortPeriod)
	ema60 := ema(closeSeries, a.cfg.EmaLongPeriod)
	curr := closeSeries[len(closeSeries)-1]
	e20 := ema20[len(ema20)-1]
	e60 := ema60[len(ema60)-1]

	score := 0.0
	status := "neutral"
	switch {
	case curr > e20 && e20 > e60:
		score = 60
		status = "bullish_alignment"
	case curr < e20 && e20 < e60:
		score = -60
		status = "bearish_alignment"
	case curr > e20 && e20 < e60:
		score = 20
		status = "potential_reversal_up"
	case curr < e20 && e20 > e60:
		score = -20
		status = "potential_reversal_down"
	}
	return score, map[string]any{
		"ema_status": status,
		"ema20":      e20,
		"ema60":      e60,
		"close":      curr,
	}
}

func (a *Analyzer) oscillatorScore(c []binance.Candle) (float64, map[string]any) {
	if len(c) < 30 {
		return 0, map[string]any{}
	}
	closeSeries := closes(c)
	highSeries := highs(c)
	lowSeries := lows(c)
	rsiValue := rsi(closeSeries, a.cfg.RsiPeriod)
	jValue := kdjJ(highSeries, lowSeries, closeSeries, a.cfg.KdjPeriod)

	score := 0.0
	if rsiValue < a.cfg.RsiOversold {
		score += 40
	} else if rsiValue > a.cfg.RsiOverbought {
		score -= 40
	}
	if jValue < a.cfg.KdjOversold {
		score += 30
	} else if jValue > a.cfg.KdjOverbought {
		score -= 30
	}
	return score, map[string]any{
		"rsi_value": rsiValue,
		"kdj_j":     jValue,
	}
}

func closes(c []binance.Candle) []float64 {
	out := make([]float64, 0, len(c))
	for _, x := range c {
		out = append(out, x.Close)
	}
	return out
}

func highs(c []binance.Candle) []float64 {
	out := make([]float64, 0, len(c))
	for _, x := range c {
		out = append(out, x.High)
	}
	return out
}

func lows(c []binance.Candle) []float64 {
	out := make([]float64, 0, len(c))
	for _, x := range c {
		out = append(out, x.Low)
	}
	return out
}

func ema(series []float64, span int) []float64 {
	out := make([]float64, len(series))
	if len(series) == 0 {
		return out
	}
	alpha := 2.0 / float64(span+1)
	out[0] = series[0]
	for i := 1; i < len(series); i++ {
		out[i] = alpha*series[i] + (1-alpha)*out[i-1]
	}
	return out
}

func rsi(series []float64, period int) float64 {
	if len(series) < period+1 {
		return 50
	}
	gain := 0.0
	loss := 0.0
	for i := len(series) - period; i < len(series); i++ {
		diff := series[i] - series[i-1]
		if diff > 0 {
			gain += diff
		} else {
			loss -= diff
		}
	}
	if loss == 0 {
		return 100
	}
	rs := (gain / float64(period)) / (loss / float64(period))
	return 100 - (100 / (1 + rs))
}

func kdjJ(high, low, close []float64, n int) float64 {
	if len(close) < n {
		return 50
	}
	windowHigh := maxSlice(high[len(high)-n:])
	windowLow := minSlice(low[len(low)-n:])
	if windowHigh == windowLow {
		return 50
	}
	rsv := 100 * (close[len(close)-1] - windowLow) / (windowHigh - windowLow)
	k := (2.0/3.0)*50 + (1.0/3.0)*rsv
	d := (2.0/3.0)*50 + (1.0/3.0)*k
	return 3*k - 2*d
}

func avgVolume(c []binance.Candle) float64 {
	if len(c) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range c {
		sum += x.Volume
	}
	return sum / float64(len(c))
}

func maxSlice(values []float64) float64 {
	best := math.Inf(-1)
	for _, v := range values {
		if v > best {
			best = v
		}
	}
	return best
}

func minSlice(values []float64) float64 {
	best := math.Inf(1)
	for _, v := range values {
		if v < best {
			best = v
		}
	}
	return best
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func detectRegime(c []binance.Candle, totalTrend, totalOsc float64) map[string]any {
	if len(c) < 30 {
		return map[string]any{"regime": "unknown", "confidence": 0}
	}
	cl := closes(c)
	r := returns(cl)
	vol := stddev(r) * 100
	regime := "choppy"
	conf := 55.0
	if math.Abs(totalTrend) >= 80 && vol < 2.5 {
		if totalTrend > 0 {
			regime = "trend_up"
		} else {
			regime = "trend_down"
		}
		conf = 75
	} else if vol >= 3.0 {
		regime = "volatile"
		conf = 70
	} else if math.Abs(totalOsc) > 60 && math.Abs(totalTrend) < 40 {
		regime = "range"
		conf = 62
	}
	return map[string]any{
		"regime":     regime,
		"confidence": conf,
		"atr_pct":    vol,
	}
}

func returns(values []float64) []float64 {
	out := []float64{}
	for i := 1; i < len(values); i++ {
		if values[i-1] == 0 {
			continue
		}
		out = append(out, (values[i]-values[i-1])/values[i-1])
	}
	return out
}

func stddev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(values)))
}

func trendStance(score float64) string {
	switch {
	case score >= 80:
		return "BULLISH"
	case score >= 20:
		return "SLIGHTLY_BULLISH"
	case score <= -80:
		return "BEARISH"
	case score <= -20:
		return "SLIGHTLY_BEARISH"
	default:
		return "NEUTRAL"
	}
}

func oscillatorStance(score float64) string {
	return trendStance(score)
}

func triggerStance(c []binance.Candle) string {
	if len(c) < 6 {
		return "NEUTRAL"
	}
	cl := closes(c)
	change := (cl[len(cl)-1] - cl[len(cl)-6]) / cl[len(cl)-6] * 100
	switch {
	case change > 0.8:
		return "BULLISH"
	case change < -0.8:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

func buildTrendSummary(total, s1h, s15m, s5m float64) string {
	return strings.TrimSpace(
		formatSummary("Trend", trendStance(total), total) +
			formatSummary("1h", trendStance(s1h), s1h) +
			formatSummary("15m", trendStance(s15m), s15m) +
			formatSummary("5m", trendStance(s5m), s5m),
	)
}

func buildSetupSummary(total float64, details map[string]any) string {
	return strings.TrimSpace(formatSummary("Oscillator", oscillatorStance(total), total) +
		" RSI=" + formatMaybe(details["rsi_value"]) +
		" KDJ_J=" + formatMaybe(details["kdj_j"]))
}

func buildTriggerSummary(c []binance.Candle) string {
	if len(c) < 6 {
		return "Insufficient 5m candles"
	}
	cl := closes(c)
	change := (cl[len(cl)-1] - cl[len(cl)-6]) / cl[len(cl)-6] * 100
	return "5m momentum " + formatMaybe(change) + "%"
}

func buildMultiPeriodSummary(totalTrend, totalOsc float64, regime map[string]any) string {
	return "Regime=" + formatMaybe(regime["regime"]) + " Trend=" + formatMaybe(totalTrend) + " Osc=" + formatMaybe(totalOsc)
}

func formatSummary(name, stance string, score float64) string {
	return name + ":" + stance + "(" + formatMaybe(score) + ") "
}

func formatMaybe(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strings.TrimRight(strings.TrimRight(fmtFloat(t), "0"), ".")
	default:
		return ""
	}
}

func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
