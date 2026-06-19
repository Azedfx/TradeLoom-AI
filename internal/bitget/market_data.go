package bitget

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type MarketData struct{}

func (m *MarketData) GetTechnicalAnalysis(symbol string) (map[string]interface{}, error) {
	cmd := exec.Command("bgc", "spot", "spot_get_ticker", "--symbol", symbol)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("bgc failed: %v, stderr: %s", err, stderr.String())
	}

	var raw struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal ticker: %w, raw: %s", err, stdout.String())
	}
	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("no ticker data for %s", symbol)
	}
	return raw.Data[0], nil
}

func (m *MarketData) GetSentiment(topic string) (map[string]interface{}, error) {
	cmd := exec.Command("bgc", "market", "sentiment", "--topic", topic)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	json.Unmarshal(output, &result)
	return result, nil
}
