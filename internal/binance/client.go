package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"llmtradebot/internal/config"
)

type Client struct {
	cfg     config.BinanceConfig
	httpCli *http.Client
}

type Candle struct {
	OpenTime         int64   `json:"open_time"`
	Open             float64 `json:"open"`
	High             float64 `json:"high"`
	Low              float64 `json:"low"`
	Close            float64 `json:"close"`
	Volume           float64 `json:"volume"`
	CloseTime        int64   `json:"close_time"`
	QuoteAssetVolume float64 `json:"quote_asset_volume"`
	Trades           int64   `json:"trades"`
	TakerBuyBase     float64 `json:"taker_buy_base"`
	TakerBuyQuote    float64 `json:"taker_buy_quote"`
}

type Price struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

type FuturesAccount struct {
	AvailableBalance      float64 `json:"available_balance"`
	TotalWalletBalance    float64 `json:"total_wallet_balance"`
	TotalMarginBalance    float64 `json:"total_margin_balance"`
	TotalUnrealizedProfit float64 `json:"total_unrealized_profit"`
}

type Position struct {
	Symbol           string  `json:"symbol"`
	PositionAmt      float64 `json:"position_amt"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	UnrealizedProfit float64 `json:"unrealized_profit"`
	Leverage         int     `json:"leverage"`
	PositionSide     string  `json:"position_side"`
}

type Order struct {
	OrderID       int64   `json:"order_id"`
	ClientOrderID string  `json:"client_order_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	Status        string  `json:"status"`
	Type          string  `json:"type"`
	AvgPrice      float64 `json:"avg_price"`
	ExecutedQty   float64 `json:"executed_qty"`
}

func New(cfg config.BinanceConfig) *Client {
	return &Client{
		cfg:     cfg,
		httpCli: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) Ready() bool {
	return c != nil && strings.TrimSpace(c.cfg.BaseURL) != ""
}

func (c *Client) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Candle, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	params.Set("limit", strconv.Itoa(limit))

	var raw [][]any
	if err := c.getPublic(ctx, "/fapi/v1/klines", params, &raw); err != nil {
		return nil, err
	}

	out := make([]Candle, 0, len(raw))
	for _, row := range raw {
		if len(row) < 11 {
			continue
		}
		out = append(out, Candle{
			OpenTime:         asInt64(row[0]),
			Open:             asFloat64(row[1]),
			High:             asFloat64(row[2]),
			Low:              asFloat64(row[3]),
			Close:            asFloat64(row[4]),
			Volume:           asFloat64(row[5]),
			CloseTime:        asInt64(row[6]),
			QuoteAssetVolume: asFloat64(row[7]),
			Trades:           asInt64(row[8]),
			TakerBuyBase:     asFloat64(row[9]),
			TakerBuyQuote:    asFloat64(row[10]),
		})
	}
	return out, nil
}

func (c *Client) GetTickerPrice(ctx context.Context, symbol string) (Price, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	var raw struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := c.getPublic(ctx, "/fapi/v1/ticker/price", params, &raw); err != nil {
		return Price{}, err
	}
	return Price{
		Symbol:    raw.Symbol,
		Price:     mustParseFloat(raw.Price),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (c *Client) GetFuturesAccount(ctx context.Context) (FuturesAccount, error) {
	var raw struct {
		AvailableBalance      string `json:"availableBalance"`
		TotalWalletBalance    string `json:"totalWalletBalance"`
		TotalMarginBalance    string `json:"totalMarginBalance"`
		TotalUnrealizedProfit string `json:"totalUnrealizedProfit"`
	}
	if err := c.getSigned(ctx, "/fapi/v2/account", url.Values{}, &raw); err != nil {
		return FuturesAccount{}, err
	}
	return FuturesAccount{
		AvailableBalance:      mustParseFloat(raw.AvailableBalance),
		TotalWalletBalance:    mustParseFloat(raw.TotalWalletBalance),
		TotalMarginBalance:    mustParseFloat(raw.TotalMarginBalance),
		TotalUnrealizedProfit: mustParseFloat(raw.TotalUnrealizedProfit),
	}, nil
}

func (c *Client) GetPosition(ctx context.Context, symbol string) (*Position, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	var raw []struct {
		Symbol           string `json:"symbol"`
		PositionAmt      string `json:"positionAmt"`
		EntryPrice       string `json:"entryPrice"`
		MarkPrice        string `json:"markPrice"`
		UnrealizedProfit string `json:"unRealizedProfit"`
		Leverage         string `json:"leverage"`
		PositionSide     string `json:"positionSide"`
	}
	if err := c.getSigned(ctx, "/fapi/v2/positionRisk", params, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	p := raw[0]
	return &Position{
		Symbol:           p.Symbol,
		PositionAmt:      mustParseFloat(p.PositionAmt),
		EntryPrice:       mustParseFloat(p.EntryPrice),
		MarkPrice:        mustParseFloat(p.MarkPrice),
		UnrealizedProfit: mustParseFloat(p.UnrealizedProfit),
		Leverage:         int(mustParseFloat(p.Leverage)),
		PositionSide:     p.PositionSide,
	}, nil
}

func (c *Client) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("leverage", strconv.Itoa(leverage))
	return c.postSigned(ctx, "/fapi/v1/leverage", params, nil)
}

func (c *Client) PlaceMarketOrder(ctx context.Context, symbol, side string, quantity float64, reduceOnly bool, positionSide string) (Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side)
	params.Set("type", "MARKET")
	params.Set("quantity", trimFloat(quantity))
	if positionSide != "" {
		params.Set("positionSide", positionSide)
	}
	if reduceOnly {
		params.Set("reduceOnly", "true")
	}
	var raw struct {
		OrderID       int64  `json:"orderId"`
		ClientOrderID string `json:"clientOrderId"`
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		Status        string `json:"status"`
		Type          string `json:"type"`
		AvgPrice      string `json:"avgPrice"`
		ExecutedQty   string `json:"executedQty"`
	}
	if err := c.postSigned(ctx, "/fapi/v1/order", params, &raw); err != nil {
		return Order{}, err
	}
	return Order{
		OrderID:       raw.OrderID,
		ClientOrderID: raw.ClientOrderID,
		Symbol:        raw.Symbol,
		Side:          raw.Side,
		Status:        raw.Status,
		Type:          raw.Type,
		AvgPrice:      mustParseFloat(raw.AvgPrice),
		ExecutedQty:   mustParseFloat(raw.ExecutedQty),
	}, nil
}

func (c *Client) SetStopLossTakeProfit(ctx context.Context, symbol string, stopLoss, takeProfit *float64, positionSide string) ([]Order, error) {
	position, err := c.GetPosition(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if position == nil || position.PositionAmt == 0 {
		return nil, nil
	}
	closeSide := "SELL"
	if position.PositionAmt < 0 {
		closeSide = "BUY"
	}
	orders := []Order{}
	if stopLoss != nil {
		order, err := c.placeConditionalOrder(ctx, symbol, closeSide, "STOP_MARKET", *stopLoss, positionSide)
		if err != nil {
			return orders, err
		}
		orders = append(orders, order)
	}
	if takeProfit != nil {
		order, err := c.placeConditionalOrder(ctx, symbol, closeSide, "TAKE_PROFIT_MARKET", *takeProfit, positionSide)
		if err != nil {
			return orders, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (c *Client) CancelAllOrders(ctx context.Context, symbol string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	return c.deleteSigned(ctx, "/fapi/v1/allOpenOrders", params, nil)
}

func (c *Client) getPublic(ctx context.Context, path string, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.cfg.BaseURL, "/")+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeResponse(resp, out)
}

func (c *Client) getSigned(ctx context.Context, path string, params url.Values, out any) error {
	if strings.TrimSpace(c.cfg.APIKey) == "" || strings.TrimSpace(c.cfg.APISecret) == "" {
		return fmt.Errorf("binance api key or secret not configured")
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	query := params.Encode()
	signature := sign(query, c.cfg.APISecret)
	if query != "" {
		query += "&signature=" + signature
	} else {
		query = "signature=" + signature
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.cfg.BaseURL, "/")+path+"?"+query, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeResponse(resp, out)
}

func (c *Client) postSigned(ctx context.Context, path string, params url.Values, out any) error {
	return c.doSigned(ctx, http.MethodPost, path, params, out)
}

func (c *Client) deleteSigned(ctx context.Context, path string, params url.Values, out any) error {
	return c.doSigned(ctx, http.MethodDelete, path, params, out)
}

func (c *Client) doSigned(ctx context.Context, method, path string, params url.Values, out any) error {
	if strings.TrimSpace(c.cfg.APIKey) == "" || strings.TrimSpace(c.cfg.APISecret) == "" {
		return fmt.Errorf("binance api key or secret not configured")
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	query := params.Encode()
	signature := sign(query, c.cfg.APISecret)
	if query != "" {
		query += "&signature=" + signature
	} else {
		query = "signature=" + signature
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.cfg.BaseURL, "/")+path+"?"+query, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return fmt.Errorf("binance request failed: %s: %s", resp.Status, string(body))
		}
		return nil
	}
	return decodeResponse(resp, out)
}

func (c *Client) placeConditionalOrder(ctx context.Context, symbol, side, orderType string, stopPrice float64, positionSide string) (Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side)
	params.Set("type", orderType)
	params.Set("stopPrice", trimFloat(stopPrice))
	params.Set("closePosition", "true")
	if positionSide != "" {
		params.Set("positionSide", positionSide)
	}
	var raw struct {
		OrderID       int64  `json:"orderId"`
		ClientOrderID string `json:"clientOrderId"`
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		Status        string `json:"status"`
		Type          string `json:"type"`
	}
	if err := c.postSigned(ctx, "/fapi/v1/order", params, &raw); err != nil {
		return Order{}, err
	}
	return Order{
		OrderID: raw.OrderID, ClientOrderID: raw.ClientOrderID, Symbol: raw.Symbol, Side: raw.Side, Status: raw.Status, Type: raw.Type,
	}, nil
}

func decodeResponse(resp *http.Response, out any) error {
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("binance request failed: %s: %s", resp.Status, string(body))
	}
	return json.Unmarshal(body, out)
}

func sign(message, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func asFloat64(v any) float64 {
	switch t := v.(type) {
	case string:
		return mustParseFloat(t)
	case float64:
		return t
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case string:
		i, _ := strconv.ParseInt(t, 10, 64)
		return i
	default:
		return 0
	}
}

func mustParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
