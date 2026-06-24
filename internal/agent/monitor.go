package agent

import (
	"fmt"
	"log"
	"math"
	"time"

	"trade/internal/bitget"
	"trade/internal/llm"
	"trade/internal/models"
	"trade/internal/store"
)

type Monitor struct {
	store          *store.MemoryStore
	marketData     *bitget.MarketData
	decisionEngine *DecisionEngine
	executor       *bitget.TradeExecutor
	mcpClient      *bitget.MCPClient
	intentParser   *llm.LLMClient
	interval       time.Duration
	defaultCapital float64
	maxRiskPercent float64
	takeProfitPct  float64
	stopLossPct    float64
	tradeMode      string
	lastPnlLog     map[string]float64
}

func NewMonitor(s *store.MemoryStore, md *bitget.MarketData, de *DecisionEngine, te *bitget.TradeExecutor, mcp *bitget.MCPClient, lp *llm.LLMClient, interval time.Duration, defaultCapital, maxRiskPercent, takeProfitPct, stopLossPct float64, tradeMode string) *Monitor {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if defaultCapital <= 0 {
		defaultCapital = 1000
	}
	if maxRiskPercent <= 0 {
		maxRiskPercent = 2
	}
	if takeProfitPct <= 0 {
		takeProfitPct = 5
	}
	if stopLossPct <= 0 {
		stopLossPct = 2
	}
	return &Monitor{
		store:          s,
		marketData:     md,
		decisionEngine: de,
		executor:       te,
		mcpClient:      mcp,
		intentParser:   lp,
		interval:       interval,
		defaultCapital: defaultCapital,
		maxRiskPercent: maxRiskPercent,
		takeProfitPct:  takeProfitPct,
		stopLossPct:    stopLossPct,
		tradeMode:      tradeMode,
		lastPnlLog:     make(map[string]float64),
	}
}

func (m *Monitor) Start() {
	log.Printf("[MONITOR] Background service started (Interval: %v)", m.interval)
	go func() {
		for {
			m.CheckPositions()
			m.CheckStrategies()
			time.Sleep(m.interval)
		}
	}()
}

func (m *Monitor) CheckStrategies() {
	strategies := m.store.GetAllStrategies()
	if len(strategies) == 0 {
		return
	}

	for _, s := range strategies {
		if s.Active && !s.Entered {
			m.evaluateEntry(s)
		}
	}
}

func (m *Monitor) evaluateEntry(s *models.Strategy) {
	symbol := s.TargetSymbol
	ticker := symbol
	if len(symbol) > 4 {
		ticker = symbol[:len(symbol)-4]
	}

	// Fetch technicals
	tech, err := m.marketData.GetTechnicalIndicators(symbol)
	if err != nil {
		log.Printf("[MONITOR] Error fetching technicals for %s: %v", symbol, err)
		return
	}

	// Fetch MCP Data
	sentiment, _ := m.mcpClient.SentimentIndex("fear-and-greed")
	news, _ := m.mcpClient.NewsFeed("cryptopanic", ticker, 5)
	macro, _ := m.mcpClient.MacroIndicators("latest", "cpi")

	analysis, _ := m.intentParser.AnalyzeMarketWithData(symbol, string(sentiment), string(news), string(macro))
	if analysis == nil {
		return
	}

	perception := &models.MarketPerception{
		Technical: map[string]interface{}{
			"trend":       tech.Trend,
			"rsi":         tech.RSI14,
			"last_price":  tech.LastPrice,
			"change24h":   fmt.Sprintf("%f", tech.Change24h),
			"quoteVolume": fmt.Sprintf("%f", tech.Volume),
		},
		Sentiment: analysis.Sentiment,
		News:      analysis.News,
		Macro:     analysis.Macro,
	}

	confidence := m.decisionEngine.EvaluateWithConfidence(perception)
	m.store.LogDecision(symbol, confidence.Total, confidence.Decision, confidence.Signals)

	if confidence.Decision == "BUY" {
		log.Printf("[MONITOR] BUY SIGNAL for %s! Confidence: %.0f/100.", symbol, confidence.Total)

		capital, _ := m.executor.GetBalance("USDT")
		if capital == 0 {
			capital = m.store.AccountBalance
		}
		maxRisk := capital * (m.maxRiskPercent / 100)
		size := maxRisk / tech.LastPrice

		if m.tradeMode != "demo" {
			m.executor.PlaceOrder(symbol, "buy", "market", size, 0)
		}
		m.store.LogTrade(symbol, "buy", size, tech.LastPrice, 0, "OPEN")
		m.store.AccountBalance -= maxRisk

		s.Entered = true
		m.store.SaveStrategy(s)
	} else {
		log.Printf("[MONITOR] ⏳ %s: Decision is %s (Score: %.0f/100). Waiting for better conditions...", symbol, confidence.Decision, confidence.Total)
	}
}

func (m *Monitor) CheckPositions() {
	openPositions := m.store.GetOpenPositions()
	if len(openPositions) == 0 {
		return
	}

	for _, p := range openPositions {
		m.evaluateExit(p)
	}
}

func (m *Monitor) evaluateExit(t models.Trade) {
	tech, err := m.marketData.GetTechnicalIndicators(t.Symbol)
	if err != nil {
		return
	}

	pnlPct := (tech.LastPrice - t.Price) / t.Price

	shouldExit := false
	reason := ""

	if m.tradeMode != "demo" && tech.Trend == "bearish" {
		shouldExit = true
		reason = "Trend turned bearish"
	} else if tech.RSI14 > 80 {
		shouldExit = true
		reason = "RSI overbought (80+)"
	} else if pnlPct > (m.takeProfitPct / 100) {
		shouldExit = true
		reason = fmt.Sprintf("Take profit target reached (+%.2f%%)", m.takeProfitPct)
	} else if pnlPct < -(m.stopLossPct / 100) {
		shouldExit = true
		reason = fmt.Sprintf("Stop loss target reached (-%.2f%%)", m.stopLossPct)
	}

	if shouldExit {
		log.Printf("[MONITOR] EXIT %s: %s | PnL: %.2f%%", t.Symbol, reason, pnlPct*100)
		delete(m.lastPnlLog, t.Symbol)
		if m.tradeMode != "demo" {
			m.executor.PlaceOrder(t.Symbol, "sell", "market", t.Size, 0)
		}
		realizedPnl := pnlPct * (t.Size * t.Price)
		m.store.CloseTrade(t.ID, tech.LastPrice, realizedPnl)
	} else {
		last, seen := m.lastPnlLog[t.Symbol]
		pnlRounded := math.Round(pnlPct*100) / 100
		if !seen || math.Abs(pnlRounded-last) > 0.5 {
			m.lastPnlLog[t.Symbol] = pnlRounded
			log.Printf("[MONITOR] %s PnL: %+.2f%%", t.Symbol, pnlPct*100)
		}
	}
}
