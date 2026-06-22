package bitget

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type MarketData struct{}

func (m *MarketData) GetTechnicalAnalysis(symbol string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.bitget.com/api/v2/spot/market/ticker?symbol=%s", symbol)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ticker request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal ticker: %w, raw: %s", err, string(body))
	}
	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("no ticker data for %s", symbol)
	}
	return raw.Data[0], nil
}
