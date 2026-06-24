package bitget

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

type MCPClient struct {
	httpClient *http.Client
}

func NewMCPClient(baseURL string) *MCPClient {
	return &MCPClient{
		httpClient: newHTTPClient(10 * time.Second),
	}
}

// SentimentIndex fetches market tickers and computes a fear-greed-like sentiment
func (c *MCPClient) SentimentIndex(action string) (json.RawMessage, error) {
	url := "https://api.bitget.com/api/v2/spot/market/tickers"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fallbackSentiment()
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var raw struct {
		Code string `json:"code"`
		Data []struct {
			Symbol    string `json:"symbol"`
			LastPr    string `json:"lastPr"`
			Change24h string `json:"change24h"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.Code != "00000" {
		return fallbackSentiment()
	}

	totalChange := 0.0
	count := 0
	for _, d := range raw.Data {
		var ch float64
		fmt.Sscanf(d.Change24h, "%f", &ch)
		totalChange += ch
		count++
	}
	avgChange := totalChange / float64(count)
	score := 50.0 + avgChange*500
	if score > 95 {
		score = 95
	} else if score < 5 {
		score = 5
	}
	label := "Neutral"
	if score > 65 {
		label = "Greed"
	} else if score > 50 {
		label = "Neutral"
	} else {
		label = "Fear"
	}

	output := map[string]interface{}{
		"score":      score / 100.0,
		"fear_greed": score,
		"label":      label,
		"source":     "Bitget Agent Hub — Market Tickers",
	}
	raw2, _ := json.Marshal(output)
	return raw2, nil
}

func fallbackSentiment() (json.RawMessage, error) {
	output := map[string]interface{}{
		"score":      0.5,
		"fear_greed": 50,
		"label":      "Neutral",
		"source":     "fallback",
	}
	raw, _ := json.Marshal(output)
	return raw, nil
}

// CryptoMarket fetches trending coins or price data from Bitget API
func (c *MCPClient) CryptoMarket(action string, params map[string]interface{}) (json.RawMessage, error) {
	if action == "trending" {
		return c.bitgetVolumeTrending()
	}
	if action == "price" {
		symbol, _ := params["symbol"].(string)
		if symbol == "" {
			symbol = "BTCUSDT"
		}
		return c.bitgetTicker(symbol)
	}
	output := map[string]interface{}{"note": "action not supported", "source": "Bitget Agent Hub"}
	raw, _ := json.Marshal(output)
	return raw, nil
}

func (c *MCPClient) bitgetVolumeTrending() (json.RawMessage, error) {
	url := "https://api.bitget.com/api/v2/spot/market/tickers"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("bitget tickers: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Code string `json:"code"`
		Data []struct {
			Symbol      string `json:"symbol"`
			LastPr      string `json:"lastPr"`
			Change24h   string `json:"change24h"`
			QuoteVolume string `json:"quoteVolume"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.Code != "00000" {
		return nil, fmt.Errorf("bitget tickers decode: %w", err)
	}

	type coin struct {
		Symbol      string  `json:"symbol"`
		Volume      float64 `json:"volume"`
		Price       string  `json:"price"`
		Change24h   string  `json:"change24h"`
	}
	var coins []coin
	for _, d := range raw.Data {
		var qv float64
		fmt.Sscanf(d.QuoteVolume, "%f", &qv)
		coins = append(coins, coin{
			Symbol:    d.Symbol,
			Volume:    qv,
			Price:     d.LastPr,
			Change24h: d.Change24h,
		})
	}

	sort.Slice(coins, func(i, j int) bool {
		return coins[i].Volume > coins[j].Volume
	})
	if len(coins) > 20 {
		coins = coins[:20]
	}

	return json.Marshal(coins)
}

func (c *MCPClient) bitgetTicker(symbol string) (json.RawMessage, error) {
	url := fmt.Sprintf("https://api.bitget.com/api/v2/spot/market/tickers?symbol=%s", symbol)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("bitget ticker: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Code string                   `json:"code"`
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.Code != "00000" {
		return nil, fmt.Errorf("bitget ticker decode: %w", err)
	}
	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("no data for %s", symbol)
	}
	return json.Marshal(raw.Data[0])
}

// NewsFeed generates market news context using Bitget market data
func (c *MCPClient) NewsFeed(feeds string, keyword string, limit int) (json.RawMessage, error) {
	topVol := 5
	if limit > 0 {
		topVol = limit
	}
	trending, err := c.bitgetVolumeTrending()
	if err != nil {
		output := map[string]interface{}{
			"articles": []interface{}{},
			"summary":  "Bitget Agent Hub — News feed derived from market data",
		}
		raw, _ := json.Marshal(output)
		return raw, nil
	}

	var coins []map[string]interface{}
	json.Unmarshal(trending, &coins)

	if topVol > len(coins) {
		topVol = len(coins)
	}
	coins = coins[:topVol]

	output := map[string]interface{}{
		"articles": coins,
		"summary":  fmt.Sprintf("Top %d trending markets from Bitget Agent Hub", topVol),
	}
	raw, _ := json.Marshal(output)
	return raw, nil
}

// MacroIndicators returns top market data as proxy for macro sentiment
func (c *MCPClient) MacroIndicators(action string, indicator string) (json.RawMessage, error) {
	trending, err := c.bitgetVolumeTrending()
	if err != nil {
		output := map[string]interface{}{
			"note":      "Macro data from Bitget Agent Hub — USDT volume analysis",
			"indicator": indicator,
		}
		raw, _ := json.Marshal(output)
		return raw, nil
	}

	var coins []map[string]interface{}
	json.Unmarshal(trending, &coins)

	totalVol := 0.0
	posChange := 0
	negChange := 0
	for _, c := range coins {
		vol, _ := c["volume"].(float64)
		totalVol += vol
		ch, _ := c["change24h"].(string)
		var chf float64
		fmt.Sscanf(ch, "%f", &chf)
		if chf > 0 {
			posChange++
		} else if chf < 0 {
			negChange++
		}
	}

	riskAppetite := "mixed"
	if posChange > negChange*2 {
		riskAppetite = "risk-on"
	} else if negChange > posChange*2 {
		riskAppetite = "risk-off"
	}

	output := map[string]interface{}{
		"score":         0.5,
		"risk_appetite": riskAppetite,
		"top_volume_usdt": totalVol / float64(len(coins)),
		"source":        "Bitget Agent Hub — Market Volume Analysis",
	}
	raw, _ := json.Marshal(output)
	return raw, nil
}

// TopVolumeCoins returns top N coins by USDT volume from Bitget
func (c *MCPClient) TopVolumeCoins(limit int) (json.RawMessage, error) {
	return c.bitgetVolumeTrending()
}
