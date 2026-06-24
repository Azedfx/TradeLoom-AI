package bitget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type MarketData struct {
	httpCli      *http.Client
	usingDemo    bool
	demoFields   []string
}

func (m *MarketData) UsingDemoData() bool {
	return m.usingDemo
}

func (m *MarketData) DemoFields() []string {
	return m.demoFields
}

func NewMarketData() *MarketData {
	dialer := &net.Dialer{Timeout: 1 * time.Second}
	return &MarketData{
		httpCli: &http.Client{
			Timeout:   3 * time.Second,
			Transport: &http.Transport{DialContext: dialer.DialContext},
		},
	}
}

func bitgetReachable() bool {
	dialer := net.Dialer{Timeout: 1 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	conn, err := dialer.DialContext(ctx, "tcp", "api.bitget.com:443")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

var reachable bool

func init() {
	reachable = bitgetReachable()
	if !reachable {
		fmt.Println("⚠ Bitget API unreachable — using demo data")
	}
}

func (m *MarketData) GetTechnicalAnalysis(symbol string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.bitget.com/api/v2/spot/market/ticker?symbol=%s", symbol)
	resp, err := m.httpCli.Get(url)
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
