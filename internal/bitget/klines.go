package bitget

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Kline struct {
	Timestamp int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

func (m *MarketData) GetKlines(symbol, granularity string, limit int) ([]Kline, error) {
	url := fmt.Sprintf("https://api.bitget.com/api/v2/spot/market/candles?symbol=%s&granularity=%s&limit=%d",
		symbol, granularity, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("klines request: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		Data [][]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("klines decode: %w", err)
	}

	klines := make([]Kline, 0, len(raw.Data))
	for _, d := range raw.Data {
		if len(d) < 6 {
			continue
		}
		k := Kline{}
		k.Timestamp, _ = strconv.ParseInt(d[0], 10, 64)
		k.Open, _ = strconv.ParseFloat(d[1], 64)
		k.High, _ = strconv.ParseFloat(d[2], 64)
		k.Low, _ = strconv.ParseFloat(d[3], 64)
		k.Close, _ = strconv.ParseFloat(d[4], 64)
		k.Volume, _ = strconv.ParseFloat(d[5], 64)
		klines = append(klines, k)
	}

	return klines, nil
}

// CalculateRSI computes RSI for the given period
func CalculateRSI(klines []Kline, period int) (float64, error) {
	if len(klines) < period+1 {
		return 0, fmt.Errorf("not enough data for RSI-%d: need %d, got %d", period, period+1, len(klines))
	}

	gains, losses := 0.0, 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100, nil
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))
	return rsi, nil
}

// CalculateSMA computes Simple Moving Average
func CalculateSMA(klines []Kline, period int) (float64, error) {
	if len(klines) < period {
		return 0, fmt.Errorf("not enough data for SMA-%d", period)
	}
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	return sum / float64(period), nil
}

// DetermineTrend compares short vs long SMA to determine trend direction
func DetermineTrend(klines []Kline) string {
	shortSMA, err1 := CalculateSMA(klines, 7)
	longSMA, err2 := CalculateSMA(klines, 25)
	if err1 != nil || err2 != nil {
		return "unknown"
	}
	if shortSMA > longSMA*1.02 {
		return "bullish"
	} else if shortSMA < longSMA*0.98 {
		return "bearish"
	}
	return "neutral"
}

type TechnicalIndicators struct {
	Symbol      string  `json:"symbol"`
	RSI14       float64 `json:"rsi_14"`
	RSI7        float64 `json:"rsi_7"`
	Trend       string  `json:"trend"`
	SMA7        float64 `json:"sma_7"`
	SMA25       float64 `json:"sma_25"`
	LastPrice   float64 `json:"last_price"`
	Change24h   float64 `json:"change_24h"`
	Volume      float64 `json:"volume"`
	Granularity string  `json:"granularity"`
	AnalyzedAt  string  `json:"analyzed_at"`
}

func (m *MarketData) GetTechnicalIndicators(symbol string) (*TechnicalIndicators, error) {
	klines, klineErr := m.GetKlines(symbol, "1day", 30)

	rsi14 := 0.0
	rsi7 := 0.0
	sma7 := 0.0
	sma25 := 0.0
	trend := "unknown"

	if klineErr == nil && len(klines) >= 15 {
		rsi14, _ = CalculateRSI(klines, 14)
		rsi7, _ = CalculateRSI(klines, 7)
		sma7, _ = CalculateSMA(klines, 7)
		sma25, _ = CalculateSMA(klines, 25)
		trend = DetermineTrend(klines)
	}

	ticker, err := m.GetTechnicalAnalysis(symbol)
	if err != nil {
		// FALLBACK: If bgc fails, use the last price from klines if available
		if len(klines) > 0 {
			lp := klines[len(klines)-1].Close
			return &TechnicalIndicators{
				Symbol:      symbol,
				RSI14:       rsi14,
				RSI7:        rsi7,
				Trend:       trend,
				SMA7:        sma7,
				SMA25:       sma25,
				LastPrice:   lp,
				Change24h:   0,
				Volume:      klines[len(klines)-1].Volume,
				Granularity: "1day",
				AnalyzedAt:  time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
		return nil, err
	}

	lastPrice, _ := strconv.ParseFloat(fmt.Sprintf("%v", ticker["lastPr"]), 64)
	change24h := 0.0
	if c, ok := ticker["change24h"].(string); ok {
		change24h, _ = strconv.ParseFloat(c, 64)
	}
	volume := 0.0
	if v, ok := ticker["quoteVolume"].(string); ok {
		volume, _ = strconv.ParseFloat(v, 64)
	}

	return &TechnicalIndicators{
		Symbol:      symbol,
		RSI14:       rsi14,
		RSI7:        rsi7,
		Trend:       trend,
		SMA7:        sma7,
		SMA25:       sma25,
		LastPrice:   lastPrice,
		Change24h:   change24h,
		Volume:      volume,
		Granularity: "1d",
		AnalyzedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}
