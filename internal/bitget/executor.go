package bitget

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type TradeExecutor struct{}

func (e *TradeExecutor) PlaceOrder(symbol string, side string, orderType string, size float64, price float64) (map[string]interface{}, error) {
	orders := []map[string]interface{}{
		{
			"symbol":    symbol,
			"side":      side,
			"orderType": orderType,
			"size":      size,
			"price":     price,
		},
	}
	ordersJSON, _ := json.Marshal(orders)

	cmd := exec.Command("bgc", "spot", "spot_place_order", "--orders", string(ordersJSON))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	json.Unmarshal(output, &result)
	return result, nil
}

func (e *TradeExecutor) CancelOrder(symbol string, orderId string) error {
	cmd := exec.Command("bgc", "spot", "spot_cancel_order", "--symbol", symbol, "--orderId", orderId)
	_, err := cmd.Output()
	return err
}

func (e *TradeExecutor) GetBalance(asset string) (float64, error) {
	cmd := exec.Command("bgc", "account", "get_account_assets")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var resp struct {
		Data []struct {
			AssetName string `json:"assetName"`
			Available string `json:"available"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		return 0, fmt.Errorf("unmarshal assets: %w", err)
	}

	for _, a := range resp.Data {
		if a.AssetName == asset {
			val, _ := strconv.ParseFloat(a.Available, 64)
			return val, nil
		}
	}

	return 0, nil
}
