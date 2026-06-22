package agent

import (
	"fmt"
	"log"
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
}

func NewMonitor(s *store.MemoryStore, md *bitget.MarketData, de *DecisionEngine, te *bitget.TradeExecutor, mcp *bitget.MCPClient, lp *llm.LLMClient) *Monitor {
	return &Monitor{
		store:          s,
		marketData:     md,
		decisionEngine: de,
		executor:       te,
		mcpClient:      mcp,
		intentParser:   lp,
		interval:       10 * time.Second,
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
		log.Printf("[MONITOR] 🔥 BUY SIGNAL for %s! Confidence: %.0f/100. Executing...", symbol, confidence.Total)

		// Risk sizing
		capital, _ := m.executor.GetBalance("USDT")
		if capital == 0 {
			capital = 1000
		}
		size := (capital * 0.02) / tech.LastPrice

		m.executor.PlaceOrder(symbol, "buy", "market", size, 0)
		m.store.LogTrade(symbol, "buy", size, tech.LastPrice, 0, "OPEN")

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
	// Fetch current technicals
	tech, err := m.marketData.GetTechnicalIndicators(t.Symbol)
	if err != nil {
		return
	}

	// 1. Exit if PnL is > 5% (Take Profit) or < -2% (Stop Loss)
	pnlPct := (tech.LastPrice - t.Price) / t.Price

	shouldExit := false
	reason := ""

	if tech.Trend == "bearish" {
		shouldExit = true
		reason = "Trend turned bearish"
	} else if tech.RSI14 > 80 {
		shouldExit = true
		reason = "RSI overbought (80+)"
	} else if pnlPct > 0.05 {
		shouldExit = true
		reason = "Take Profit target reached (+5%)"
	} else if pnlPct < -0.02 {
		shouldExit = true
		reason = "Stop Loss target reached (-2%)"
	}

	if shouldExit {
		log.Printf("[MONITOR] 🛑 EXIT SIGNAL for %s: %s (PnL: %.2f%%). Closing position...", t.Symbol, reason, pnlPct*100)
		m.executor.PlaceOrder(t.Symbol, "sell", "market", t.Size, 0)
		m.store.CloseTrade(t.ID, tech.LastPrice, pnlPct*(t.Size*t.Price))
	} else {
		log.Printf("[MONITOR] 🔎 Monitoring %s position. Current PnL: %.2f%%. Holding...", t.Symbol, pnlPct*100)
	}
}
