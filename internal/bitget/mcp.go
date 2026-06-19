package bitget

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type MCPClient struct {
	baseURL   string
	httpClient *http.Client
}

func NewMCPClient(baseURL string) *MCPClient {
	return &MCPClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *MCPClient) call(method string, params interface{}) (json.RawMessage, error) {
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST", c.baseURL, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 400 {
		var errResp struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &errResp) == nil && errResp.Error != nil {
			return nil, fmt.Errorf("mcp: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("mcp: %s", string(raw))
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("mcp returned %d: %s", resp.StatusCode, string(raw))
	}

	var mcpResp mcpResponse
	if err := json.Unmarshal(raw, &mcpResp); err != nil {
		return nil, fmt.Errorf("mcp unmarshal: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}

	return mcpResp.Result, nil
}

// tools_call calls an MCP tool with given arguments
func (c *MCPClient) toolsCall(name string, args map[string]interface{}) (json.RawMessage, error) {
	return c.call("tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
}

// SentimentIndex fetches fear & greed index
func (c *MCPClient) SentimentIndex(action string) (json.RawMessage, error) {
	return c.toolsCall("sentiment_index", map[string]interface{}{
		"action": action,
	})
}

// DerivativesSentiment fetches long/short ratio and other derivatives data
func (c *MCPClient) DerivativesSentiment(action, symbol, period string) (json.RawMessage, error) {
	args := map[string]interface{}{"action": action, "period": period}
	if symbol != "" {
		args["symbol"] = symbol
	}
	return c.toolsCall("derivatives_sentiment", args)
}

// CryptoMarket fetches market data (price, global, trending, etc.)
func (c *MCPClient) CryptoMarket(action string, params map[string]interface{}) (json.RawMessage, error) {
	args := map[string]interface{}{"action": action}
	for k, v := range params {
		args[k] = v
	}
	return c.toolsCall("crypto_market", args)
}

// NewsFeed fetches latest news
func (c *MCPClient) NewsFeed(feeds string, keyword string, limit int) (json.RawMessage, error) {
	args := map[string]interface{}{
		"action": "latest",
		"feeds":  feeds,
		"limit":  limit,
	}
	if keyword != "" {
		args["keyword"] = keyword
	}
	return c.toolsCall("news_feed", args)
}

// MacroIndicators fetches economic indicators
func (c *MCPClient) MacroIndicators(action string, indicator string) (json.RawMessage, error) {
	return c.toolsCall("macro_indicators", map[string]interface{}{
		"action":    action,
		"indicator": indicator,
	})
}

// GlobalAssets fetches global asset prices (DXY, gold, S&P, etc.)
func (c *MCPClient) GlobalAssets(action, symbol string) (json.RawMessage, error) {
	return c.toolsCall("global_assets", map[string]interface{}{
		"action": action,
		"symbol": symbol,
	})
}
