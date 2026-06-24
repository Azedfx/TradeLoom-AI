package bitget

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type MarketData struct {
	httpCli *http.Client
}

func NewMarketData() *MarketData {
	return &MarketData{
		httpCli: newHTTPClient(5 * time.Second),
	}
}

func (m *MarketData) GetTechnicalAnalysis(symbol string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.bitget.com/api/v2/spot/market/tickers?symbol=%s", symbol)
	resp, err := m.httpCli.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ticker request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Code string                   `json:"code"`
		Msg  string                   `json:"msg"`
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal ticker: %w, raw: %s", err, string(body))
	}
	if raw.Code != "00000" || len(raw.Data) == 0 {
		return nil, fmt.Errorf("bitget ticker error: %s - %s", raw.Code, raw.Msg)
	}
	return raw.Data[0], nil
}
