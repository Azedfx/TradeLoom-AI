package bitget

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"sync"
	"time"
)

var (
	demoWarned   sync.Map
	demoPriceSeed sync.Map
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
	if !reachable {
		return nil, fmt.Errorf("bitget API unreachable")
	}
	url := fmt.Sprintf("https://api.bitget.com/api/v2/spot/market/candles?symbol=%s&granularity=%s&limit=%d",
		symbol, granularity, limit)

	resp, err := m.httpCli.Get(url)
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
	lastPrice := 0.0
	volume := 0.0

	m.usingDemo = false
	m.demoFields = nil

	if klineErr == nil && len(klines) >= 15 {
		rsi14, _ = CalculateRSI(klines, 14)
		rsi7, _ = CalculateRSI(klines, 7)
		sma7, _ = CalculateSMA(klines, 7)
		sma25, _ = CalculateSMA(klines, 25)
		trend = DetermineTrend(klines)
		lastPrice = klines[len(klines)-1].Close
		volume = klines[len(klines)-1].Volume
	}

	change24h := 0.0
	ticker, err := m.GetTechnicalAnalysis(symbol)
	if err == nil {
		if lp, ok := ticker["lastPr"].(string); ok {
			lastPrice, _ = strconv.ParseFloat(lp, 64)
		}
		if c, ok := ticker["change24h"].(string); ok {
			change24h, _ = strconv.ParseFloat(c, 64)
		}
		if v, ok := ticker["quoteVolume"].(string); ok {
			volume, _ = strconv.ParseFloat(v, 64)
		}
	}

	if klineErr != nil || lastPrice == 0 || volume == 0 || rsi14 == 0 {
		m.usingDemo = true
	}

	if m.usingDemo {
		if _, loaded := demoWarned.LoadOrStore(symbol, true); !loaded {
			log.Printf("[DEMO] ⚠ %s: API unreachable — using simulated data", symbol)
		}
	}

	if lastPrice == 0 {
		lastPrice = demoPrice(symbol)
		m.demoFields = append(m.demoFields, "price")
	}
	if volume == 0 {
		volume = demoVolume(symbol)
		m.demoFields = append(m.demoFields, "volume")
	}
	if change24h == 0 {
		change24h = demoChange24h(symbol)
		m.demoFields = append(m.demoFields, "change24h")
	}
	if rsi14 == 0 {
		rsi14, rsi7 = demoRSI(symbol)
		m.demoFields = append(m.demoFields, "rsi")
	}
	if trend == "unknown" {
		trend = demoTrend(symbol)
		m.demoFields = append(m.demoFields, "trend")
	}
	if sma7 == 0 || sma25 == 0 {
		sma7 = lastPrice * 0.99
		sma25 = lastPrice * 0.96
		m.demoFields = append(m.demoFields, "sma")
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

func demoPrice(symbol string) float64 {
	prices := map[string]float64{
		"BTCUSDT": 68500, "ETHUSDT": 3450, "SOLUSDT": 148,
		"FETUSDT": 1.82, "TAOUSDT": 420, "RNDRUSDT": 9.50,
		"RENDERUSDT": 9.50, "ARBUSDT": 1.05, "OPUSDT": 2.80,
		"LINKUSDT": 18.50, "AVAXUSDT": 38.00, "DOTUSDT": 7.20,
		"MATICUSDT": 0.72, "UNIUSDT": 7.80, "ATOMUSDT": 8.90,
		"NEARUSDT": 5.80, "AGIXUSDT": 0.85, "ONDOUSDT": 1.20,
		"SUIUSDT": 2.15, "APTUSDT": 9.40, "STRKUSDT": 0.65,
		"DOGEUSDT": 0.14, "SHIBUSDT": 0.000025, "PEPEUSDT": 0.000012,
		"WIFUSDT": 2.45, "AAVEUSDT": 145, "MKRUSDT": 1800,
		"CRVUSDT": 0.45, "IMXUSDT": 1.80, "SANDUSDT": 0.42,
		"GALAUSDT": 0.038, "CFGUSDT": 0.35,
	}
	base, ok := prices[symbol]
	if !ok {
		h := 0
		for _, c := range symbol {
			h = h*31 + int(c)
		}
		bases := []float64{0.35, 1.20, 3.60, 8.50, 18.00, 45.00, 120.00, 350.00}
		base = bases[h%len(bases)] * (1.0 + float64(h%20)/100.0)
	}
	// Add time-based drift so prices fluctuate for demo simulation
	now := float64(time.Now().Unix())
	drift := math.Sin(now/300.0+float64(hashStr(symbol))) * 0.03
	return base * (1.0 + drift)
}

func hashStr(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	return h
}

func demoVolume(symbol string) float64 {
	vols := map[string]float64{
		"BTCUSDT": 2.5e9, "ETHUSDT": 1.8e9, "SOLUSDT": 1.2e9,
		"FETUSDT": 3.2e8, "TAOUSDT": 1.8e8, "RNDRUSDT": 2.1e8,
	}
	if v, ok := vols[symbol]; ok {
		return v
	}
	// Deterministic volume by symbol hash
	h := 0
	for _, c := range symbol {
		h = h*31 + int(c)
	}
	return float64(5e7 + (h%10)*4e7)
}

func demoRSI(symbol string) (rsi14, rsi7 float64) {
	h := float64(hashStr(symbol))
	now := float64(time.Now().Unix())
	// RSI oscillates between 25-75 with a symbol-specific frequency
	rsi14 = 50.0 + math.Sin(now/200.0+h*0.7)*25.0
	if rsi14 > 82 {
		rsi14 = 82
	} else if rsi14 < 18 {
		rsi14 = 18
	}
	rsi7 = rsi14 + math.Sin(now/120.0+h*0.3)*5.0
	return
}

func demoTrend(symbol string) string {
	h := float64(hashStr(symbol))
	now := float64(time.Now().Unix())
	val := math.Sin(now/250.0 + h*0.5)
	if val > 0.3 {
		return "bullish"
	} else if val < -0.3 {
		return "bearish"
	}
	return "neutral"
}

func demoChange24h(symbol string) float64 {
	changes := map[string]float64{
		"FETUSDT": 0.038, "TAOUSDT": 0.052, "RNDRUSDT": 0.045,
		"SOLUSDT": 0.021, "ETHUSDT": 0.015, "BTCUSDT": 0.008,
	}
	if v, ok := changes[symbol]; ok {
		return v
	}
	// Mix of positive and negative changes
	h := 0
	for _, c := range symbol {
		h = h*31 + int(c)
	}
	if h%3 == 0 {
		return -float64(1+h%5) / 100.0
	}
	return float64(1+h%6) / 100.0
}
