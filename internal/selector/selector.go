package selector

import (
	"context"
	"sort"

	"llmtradebot/internal/binance"
)

type Agent struct {
	client     *binance.Client
	candidates []string
}

type Result struct {
	Mode    string         `json:"mode"`
	Symbols []string       `json:"symbols"`
	Scores  map[string]any `json:"scores"`
}

func New(client *binance.Client, candidates []string) *Agent {
	if len(candidates) == 0 {
		candidates = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT"}
	}
	return &Agent{client: client, candidates: candidates}
}

func (a *Agent) SelectAuto1(ctx context.Context) (Result, error) {
	type scoreRow struct {
		symbol string
		score  float64
		detail map[string]any
	}
	rows := make([]scoreRow, 0, len(a.candidates))
	for _, sym := range a.candidates {
		klines, err := a.client.GetKlines(ctx, sym, "1m", 60)
		if err != nil || len(klines) < 30 {
			continue
		}
		cl0 := klines[len(klines)-31].Close
		cl1 := klines[len(klines)-1].Close
		changePct := (cl1 - cl0) / cl0 * 100
		volShort := avgVol(klines[len(klines)-5:])
		volLong := avgVol(klines[len(klines)-30 : len(klines)-5])
		rvol := 0.0
		if volLong > 0 {
			rvol = volShort / volLong
		}
		score := abs(changePct) * maxFloat(rvol, 0.5)
		rows = append(rows, scoreRow{
			symbol: sym,
			score:  score,
			detail: map[string]any{"change_pct": changePct, "rvol": rvol, "score": score},
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	symbols := []string{}
	scores := map[string]any{}
	for i, row := range rows {
		if i >= 3 {
			break
		}
		symbols = append(symbols, row.symbol)
		scores[row.symbol] = row.detail
	}
	return Result{Mode: "AUTO1", Symbols: symbols, Scores: scores}, nil
}

func avgVol(c []binance.Candle) float64 {
	if len(c) == 0 {
		return 0
	}
	sum := 0.0
	for _, k := range c {
		sum += k.Volume
	}
	return sum / float64(len(c))
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
