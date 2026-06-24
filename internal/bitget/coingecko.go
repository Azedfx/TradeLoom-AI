package bitget

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var tickerToCoinGecko = map[string]string{
	"BTCUSDT": "bitcoin", "ETHUSDT": "ethereum", "SOLUSDT": "solana",
	"FETUSDT": "fetch-ai", "TAOUSDT": "bittensor", "RNDRUSDT": "render-token",
	"RENDERUSDT": "render-token", "ARBUSDT": "arbitrum", "OPUSDT": "optimism",
	"LINKUSDT": "chainlink", "AVAXUSDT": "avalanche-2", "DOTUSDT": "polkadot",
	"MATICUSDT": "matic-network", "UNIUSDT": "uniswap", "ATOMUSDT": "cosmos",
	"NEARUSDT": "near", "AGIXUSDT": "singularitynet", "ONDOUSDT": "ondo-finance",
	"SUIUSDT": "sui", "APTUSDT": "aptos", "STRKUSDT": "starknet",
	"DOGEUSDT": "dogecoin", "SHIBUSDT": "shiba-inu", "PEPEUSDT": "pepe",
	"WIFUSDT": "dogwifcoin", "AAVEUSDT": "aave", "MKRUSDT": "maker",
	"CRVUSDT": "curve-dao-token", "IMXUSDT": "immutable-x", "SANDUSDT": "the-sandbox",
	"GALAUSDT": "gala", "WLDUSDT": "worldcoin-wld", "SEIUSDT": "sei-network",
	"INJUSDT": "injective-protocol", "TIAUSDT": "celestia", "CFGUSDT": "centrifuge",
}

type CoinGeckoSource struct {
	httpCli *http.Client
}

func NewCoinGeckoSource() *CoinGeckoSource {
	return &CoinGeckoSource{
		httpCli: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CoinGeckoSource) coinID(symbol string) (string, bool) {
	id, ok := tickerToCoinGecko[strings.ToUpper(symbol)]
	return id, ok
}

func (c *CoinGeckoSource) GetKlines(symbol string) ([]Kline, error) {
	coinID, ok := c.coinID(symbol)
	if !ok {
		return nil, fmt.Errorf("coingecko: unknown coin %s", symbol)
	}
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s/ohlc?days=30&vs_currency=usd", coinID)
	resp, err := c.httpCli.Get(url)
	if err != nil {
		return nil, fmt.Errorf("coingecko ohlc: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("coingecko ohlc returned %d: %s", resp.StatusCode, string(body))
	}
	var raw [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("coingecko ohlc decode: %w", err)
	}
	klines := make([]Kline, 0, len(raw))
	for _, d := range raw {
		if len(d) < 5 {
			continue
		}
		ts, _ := d[0].(float64)
		open, _ := strconv.ParseFloat(fmt.Sprintf("%v", d[1]), 64)
		high, _ := strconv.ParseFloat(fmt.Sprintf("%v", d[2]), 64)
		low, _ := strconv.ParseFloat(fmt.Sprintf("%v", d[3]), 64)
		close_, _ := strconv.ParseFloat(fmt.Sprintf("%v", d[4]), 64)
		k := Kline{
			Timestamp: int64(ts) / 1000,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close_,
			Volume:    0,
		}
		klines = append(klines, k)
	}
	return klines, nil
}

func (c *CoinGeckoSource) GetTicker(symbol string) (map[string]interface{}, error) {
	coinID, ok := c.coinID(symbol)
	if !ok {
		return nil, fmt.Errorf("coingecko: unknown coin %s", symbol)
	}
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd&include_24hr_change=true", coinID)
	resp, err := c.httpCli.Get(url)
	if err != nil {
		return nil, fmt.Errorf("coingecko price: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("coingecko price returned %d", resp.StatusCode)
	}
	var raw map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("coingecko price decode: %w", err)
	}
	coinData, ok := raw[coinID]
	if !ok {
		return nil, fmt.Errorf("coingecko: no data for %s", coinID)
	}
	price := coinData["usd"]
	change24h := coinData["usd_24h_change"] / 100.0
	return map[string]interface{}{
		"lastPr":    fmt.Sprintf("%f", price),
		"change24h": fmt.Sprintf("%f", change24h),
	}, nil
}
