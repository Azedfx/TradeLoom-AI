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
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SentimentIndex fetches fear & greed index from Alternative.me
func (c *MCPClient) SentimentIndex(action string) (json.RawMessage, error) {
	resp, err := c.httpClient.Get("https://api.alternative.me/fng/?limit=1")
	if err != nil {
		return nil, fmt.Errorf("fng request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data []struct {
			Value        string `json:"value"`
			Classification string `json:"value_classification"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("fng unmarshal: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no fng data")
	}

	output := map[string]interface{}{
		"score":     result.Data[0].Value,
		"sentiment": result.Data[0].Classification,
	}
	raw, _ := json.Marshal(output)
	return raw, nil
}

// DerivativesSentiment fetches long/short ratio — returns empty (no free API available)
func (c *MCPClient) DerivativesSentiment(action, symbol, period string) (json.RawMessage, error) {
	output := map[string]interface{}{
		"long_short_ratio": "N/A",
		"summary":          "Derivatives data unavailable via free APIs",
	}
	raw, _ := json.Marshal(output)
	return raw, nil
}

// CryptoMarket returns trending coins from CoinGecko, or tickers from Bitget
func (c *MCPClient) CryptoMarket(action string, params map[string]interface{}) (json.RawMessage, error) {
	if action == "trending" {
		return c.coingeckoTrending()
	}
	if action == "price" {
		symbol, _ := params["symbol"].(string)
		if symbol == "" {
			symbol = "BTCUSDT"
		}
		return c.bitgetTicker(symbol)
	}
	output := map[string]interface{}{"note": "action not supported via free APIs"}
	raw, _ := json.Marshal(output)
	return raw, nil
}

func (c *MCPClient) coingeckoTrending() (json.RawMessage, error) {
	resp, err := c.httpClient.Get("https://api.coingecko.com/api/v3/search/trending")
	if err != nil {
		return nil, fmt.Errorf("trending request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Coins []struct {
			Item struct {
				ID     string `json:"id"`
				Symbol string `json:"symbol"`
				Name   string `json:"name"`
			} `json:"item"`
		} `json:"coins"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("trending unmarshal: %w", err)
	}

	var coins []map[string]interface{}
	for _, c := range result.Coins {
		coins = append(coins, map[string]interface{}{
			"symbol": c.Item.Symbol,
			"name":   c.Item.Name,
			"id":     c.Item.ID,
		})
	}
	raw, _ := json.Marshal(coins)
	return raw, nil
}

func (c *MCPClient) bitgetTicker(symbol string) (json.RawMessage, error) {
	url := fmt.Sprintf("https://api.bitget.com/api/v2/spot/market/ticker?symbol=%s", symbol)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("bitget ticker: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("no data for %s", symbol)
	}
	return json.Marshal(raw.Data[0])
}

// NewsFeed returns empty — no free news API available reliably
func (c *MCPClient) NewsFeed(feeds string, keyword string, limit int) (json.RawMessage, error) {
	output := map[string]interface{}{
		"articles": []interface{}{},
		"summary":  "News feed unavailable via free APIs",
	}
	raw, _ := json.Marshal(output)
	return raw, nil
}

// MacroIndicators returns empty — no free macro API available
func (c *MCPClient) MacroIndicators(action string, indicator string) (json.RawMessage, error) {
	output := map[string]interface{}{
		"note":    "Macro data unavailable via free APIs",
		"indicator": indicator,
	}
	raw, _ := json.Marshal(output)
	return raw, nil
}

// GlobalAssets returns empty — no free global assets API available
func (c *MCPClient) GlobalAssets(action, symbol string) (json.RawMessage, error) {
	output := map[string]interface{}{
		"symbol": symbol,
		"price":  "N/A",
	}
	raw, _ := json.Marshal(output)
	return raw, nil
}

// TopVolumeCoins fetches top N coins by 24h USDT volume from Bitget spot market
func (c *MCPClient) TopVolumeCoins(limit int) (json.RawMessage, error) {
	resp, err := c.httpClient.Get("https://api.bitget.com/api/v2/spot/market/tickers")
	if err != nil {
		return nil, fmt.Errorf("tickers request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data []struct {
			Symbol      string `json:"symbol"`
			LastPr      string `json:"lastPr"`
			Change24h   string `json:"change24h"`
			QuoteVolume string `json:"quoteVolume"`
			BaseVolume  string `json:"baseVolume"`
			High24h     string `json:"high24h"`
			Low24h      string `json:"low24h"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	type coin struct {
		Symbol      string
		QuoteVolume float64
		Change24h   float64
	}
	var coins []coin
	for _, d := range result.Data {
		if len(d.Symbol) < 4 {
			continue
		}
		// Only USDT pairs
		suffix := d.Symbol[len(d.Symbol)-4:]
		if suffix != "USDT" && suffix != "USDC" {
			continue
		}
		var qv, ch float64
		fmt.Sscanf(d.QuoteVolume, "%f", &qv)
		fmt.Sscanf(d.Change24h, "%f", &ch)
		coins = append(coins, coin{Symbol: d.Symbol, QuoteVolume: qv, Change24h: ch})
	}

	sort.Slice(coins, func(i, j int) bool {
		return coins[i].QuoteVolume > coins[j].QuoteVolume
	})

	if limit <= 0 || limit > len(coins) {
		limit = len(coins)
	}
	coins = coins[:limit]

	var out []map[string]interface{}
	for _, c := range coins {
		out = append(out, map[string]interface{}{
			"symbol":     c.Symbol,
			"volume":     c.QuoteVolume,
			"change24h":  c.Change24h,
		})
	}
	return json.Marshal(out)
}
