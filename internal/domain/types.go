package domain

type DecisionRequest struct {
	Symbol            string         `json:"symbol"`
	CurrentPrice      float64        `json:"current_price"`
	QuantAnalysis     map[string]any `json:"quant_analysis"`
	PredictResult     map[string]any `json:"predict_result"`
	ReflectionText    string         `json:"reflection_text"`
	CurrentPosition   map[string]any `json:"current_position"`
	RegimeInfo        map[string]any `json:"regime_info"`
	SelectedAgentData map[string]any `json:"selected_agent_data"`
}

type Perspective struct {
	Stance     string `json:"stance"`
	Reasons    string `json:"reasons"`
	Confidence int    `json:"confidence"`
}

type Decision struct {
	Action          string      `json:"action"`
	Confidence      float64     `json:"confidence"`
	Leverage        float64     `json:"leverage,omitempty"`
	PositionSizePct float64     `json:"position_size_pct"`
	StopLossPct     float64     `json:"stop_loss_pct,omitempty"`
	TakeProfitPct   float64     `json:"take_profit_pct,omitempty"`
	Reasoning       string      `json:"reasoning"`
	BullPerspective Perspective `json:"bull_perspective"`
	BearPerspective Perspective `json:"bear_perspective"`
	RawResponse     string      `json:"raw_response,omitempty"`
}
