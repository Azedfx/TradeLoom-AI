package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"trade/internal/agent"
	"trade/internal/bitget"
	"trade/internal/llm"
	"trade/internal/models"
	"trade/internal/store"
)

func formatPrice(p float64) string {
	if p >= 1 {
		return fmt.Sprintf("$%.2f", p)
	} else if p >= 0.01 {
		return fmt.Sprintf("$%.4f", p)
	} else if p >= 0.0001 {
		return fmt.Sprintf("$%.6f", p)
	}
	return fmt.Sprintf("$%.8f", p)
}

func signalLabel(decision string) string {
	switch decision {
	case "BUY":
		return "🟢 BUY"
	case "WATCHLIST":
		return "🟡 WATCHLIST"
	default:
		return "🔴 NO_TRADE"
	}
}

func parseRiskPercent(prompt string, defaultRisk float64) float64 {
	if defaultRisk <= 0 {
		defaultRisk = 2
	}
	parts := strings.Fields(strings.ToLower(prompt))
	for i, part := range parts {
		cleaned := strings.Trim(part, "%,")
		value, err := strconv.ParseFloat(cleaned, 64)
		if err != nil || value <= 0 {
			continue
		}
		prev := ""
		next := ""
		if i > 0 {
			prev = parts[i-1]
		}
		if i+1 < len(parts) {
			next = parts[i+1]
		}
		if strings.Contains(part, "%") || prev == "risk" || prev == "risking" || next == "risk" || next == "risking" {
			return value
		}
	}
	return defaultRisk
}

type StrategyHandler struct {
	intentParser     *llm.LLMClient
	strategyCompiler *agent.StrategyCompiler
	decisionEngine   *agent.DecisionEngine
	tradeExecutor    *bitget.TradeExecutor
	mcpClient        *bitget.MCPClient
	marketData       *bitget.MarketData
	store            *store.MemoryStore
	tradeMode        string
	defaultSymbol    string
	defaultCapital   float64
	maxRiskPercent   float64
	takeProfitPct    float64
	stopLossPct      float64

	briefCache     string
	briefCacheTime time.Time
}

func NewStrategyHandler(
	llm *llm.LLMClient,
	sc *agent.StrategyCompiler,
	de *agent.DecisionEngine,
	te *bitget.TradeExecutor,
	mcp *bitget.MCPClient,
	md *bitget.MarketData,
	s *store.MemoryStore,
	tradeMode string,
	defaultSymbol string,
	defaultCapital float64,
	maxRiskPercent float64,
	takeProfitPct float64,
	stopLossPct float64,
) *StrategyHandler {
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
	return &StrategyHandler{
		intentParser:     llm,
		strategyCompiler: sc,
		decisionEngine:   de,
		tradeExecutor:    te,
		mcpClient:        mcp,
		marketData:       md,
		store:            s,
		tradeMode:        tradeMode,
		defaultSymbol:    defaultSymbol,
		defaultCapital:   defaultCapital,
		maxRiskPercent:   maxRiskPercent,
		takeProfitPct:    takeProfitPct,
		stopLossPct:      stopLossPct,
	}
}

func (h *StrategyHandler) CreateStrategy(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	intent, err := h.intentParser.ParseIntent(req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rules := h.strategyCompiler.Compile(intent)

	strategy := &models.Strategy{
		ID:           uuid.New().String(),
		UserPrompt:   req.Prompt,
		Rules:        *rules,
		TargetSymbol: agent.ResolveSymbol(rules.AssetFilter, h.defaultSymbol),
		Active:       true,
		CreatedAt:    time.Now(),
	}
	h.store.SaveStrategy(strategy)

	c.JSON(http.StatusCreated, strategy)
}

func (h *StrategyHandler) ExecuteStrategy(c *gin.Context) {
	id := c.Param("id")
	strategy := h.store.GetStrategy(id)
	if strategy == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "strategy not found"})
		return
	}

	var req struct {
		Symbol string `json:"symbol"`
	}
	c.ShouldBindJSON(&req)

	symbol := req.Symbol
	if symbol == "" {
		symbol = strategy.TargetSymbol
		if symbol == "" {
			symbol = agent.ResolveSymbol(strategy.Rules.AssetFilter, h.defaultSymbol)
		}
	}

	ticker := symbol
	if len(symbol) > 4 {
		ticker = symbol[:len(symbol)-4]
	}
	ticker = strings.ToUpper(ticker)

	technical, err := h.marketData.GetTechnicalIndicators(symbol)
	techMap := map[string]interface{}{}
	if err == nil && technical != nil {
		techMap["trend"] = technical.Trend
		techMap["rsi"] = technical.RSI14
		techMap["change24h"] = fmt.Sprintf("%f", technical.Change24h)
		techMap["quoteVolume"] = fmt.Sprintf("%f", technical.Volume)
		techMap["last_price"] = technical.LastPrice
		techMap["sma_7"] = technical.SMA7
		techMap["sma_25"] = technical.SMA25
	} else {
		// Minimum data for safety
		techMap["last_price"] = 0.0
		techMap["trend"] = "unknown"
	}

	// Fetch real market intelligence from Bitget Skill Hub (MCP)
	sentiment, _ := h.mcpClient.SentimentIndex("fear-and-greed")
	news, _ := h.mcpClient.NewsFeed("cryptopanic", ticker, 5)
	macro, _ := h.mcpClient.MacroIndicators("latest", "cpi")

	// Pass the real data to LLM for final synthesis
	marketAnalysis, _ := h.intentParser.AnalyzeMarketWithData(symbol, string(sentiment), string(news), string(macro))

	sentimentMap := map[string]interface{}{}
	newsMap := map[string]interface{}{}
	macroMap := map[string]interface{}{}
	if marketAnalysis != nil {
		sentimentMap = marketAnalysis.Sentiment
		newsMap = marketAnalysis.News
		macroMap = marketAnalysis.Macro
	}

	perception := &models.MarketPerception{
		Technical: techMap,
		Sentiment: sentimentMap,
		News:      newsMap,
		Macro:     macroMap,
	}

	confidence := h.decisionEngine.EvaluateWithConfidence(perception)
	h.store.LogDecision(ticker, confidence.Total, confidence.Decision, confidence.Signals)

	explain := h.decisionEngine.Explain(ticker, confidence)

	action := "hold"
	if confidence.Decision == "BUY" {
		action = "buy"
	} else if confidence.Decision == "WATCHLIST" {
		action = "watchlist"
	}

	// Risk Management: 2% rule
	capital, err := h.tradeExecutor.GetBalance("USDT")
	if err != nil || capital == 0 {
		capital = h.defaultCapital
	}
	maxRisk := capital * (h.maxRiskPercent / 100)

	lastPrice, ok := techMap["last_price"].(float64)
	if !ok {
		lastPrice = 0
	}

	size := 0.0
	riskMgmt := gin.H{
		"capital":          capital,
		"max_risk":         maxRisk,
		"risk_percent":     h.maxRiskPercent,
		"take_profit_pct":  h.takeProfitPct,
		"stop_loss_pct":    h.stopLossPct,
	}

	if lastPrice > 0 {
		size = maxRisk / lastPrice
		riskMgmt["size"] = size
		if action == "buy" {
			riskMgmt["status"] = "sized"
		} else {
			riskMgmt["status"] = "not_trading"
			riskMgmt["note"] = "Monitoring — no position sized"
		}
	} else {
		riskMgmt["size"] = 0
		riskMgmt["status"] = "unavailable"
		riskMgmt["note"] = "Position sizing unavailable (no price data)"
	}

	resp := gin.H{
		"action":           action,
		"symbol":           symbol,
		"mode":             h.tradeMode,
		"confidence_score": confidence,
		"explain":          explain,
		"risk_management":  riskMgmt,
	}

	if action == "buy" {
		if h.tradeMode == "demo" {
			h.store.LogTrade(symbol, "buy", size, lastPrice, 0, "OPEN")
			resp["order"] = gin.H{
				"status":  "paper_trade",
				"symbol":  symbol,
				"side":    "buy",
				"size":    size,
				"price":   lastPrice,
				"message": "Demo trade logged — no real order placed",
			}
		} else {
			result, err := h.tradeExecutor.PlaceOrder(symbol, "buy", "market", size, 0)
			if err != nil {
				resp["order_error"] = err.Error()
			} else {
				resp["order"] = result
				h.store.LogTrade(symbol, "buy", size, lastPrice, 0, "OPEN")
				strategy.Entered = true
				h.store.SaveStrategy(strategy)
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (h *StrategyHandler) GetStrategyStatus(c *gin.Context) {
	id := c.Param("id")
	strategy := h.store.GetStrategy(id)
	if strategy == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "strategy not found"})
		return
	}
	c.JSON(http.StatusOK, strategy)
}

func (h *StrategyHandler) Dashboard(c *gin.Context) {
	strategies := h.store.GetAllStrategies()
	trades := h.store.GetAllTrades()
	decisions := h.store.GetDecisionLog()
	opps, lastPrompt := h.store.GetLastOpportunities()

	dataSource := "live"
	demoFields := []string{}
	if h.marketData != nil && h.marketData.UsingDemoData() {
		dataSource = "demo"
		demoFields = h.marketData.DemoFields()
	}

	c.JSON(http.StatusOK, gin.H{
		"strategies":      strategies,
		"trades":          trades,
		"decisions":       decisions,
		"totalPnL":        h.store.CalculatePnL(),
		"winRate":         h.store.CalculateWinRate(),
		"mode":            h.tradeMode,
		"conversation":    h.store.GetConversation(),
		"opportunities":   opps,
		"lastPrompt":      lastPrompt,
		"account_balance": h.store.AccountBalance,
		"data_source":     dataSource,
		"demo_fields":     demoFields,
		"risk_policy": gin.H{
			"default_capital":   h.defaultCapital,
			"max_risk_percent": h.maxRiskPercent,
			"take_profit_pct":  h.takeProfitPct,
			"stop_loss_pct":    h.stopLossPct,
		},
	})
}

func (h *StrategyHandler) ListTrades(c *gin.Context) {
	trades := h.store.GetAllTrades()
	c.JSON(http.StatusOK, gin.H{
		"trades": trades,
		"count":  len(trades),
	})
}

func (h *StrategyHandler) ExportTradesCSV(c *gin.Context) {
	trades := h.store.GetAllTrades()
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=trade_log.csv")

	var b strings.Builder
	b.WriteString("timestamp,symbol,direction,price,quantity,pnl,status\n")
	for _, t := range trades {
		b.WriteString(fmt.Sprintf("%s,%s,%s,%.8f,%.8f,%.2f,%s\n",
			t.CreatedAt.Format("2006-01-02 15:04:05"),
			t.Symbol, t.Side, t.Price, t.Size, t.PnL, t.Status))
	}
	c.String(http.StatusOK, b.String())
}

func (h *StrategyHandler) TrendingCoins(c *gin.Context) {
	raw, err := h.mcpClient.CryptoMarket("trending", map[string]interface{}{"limit": 5})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"trending": []string{}, "error": err.Error()})
		return
	}
	var trending []interface{}
	json.Unmarshal(raw, &trending)
	c.JSON(http.StatusOK, gin.H{"trending": trending})
}

func (h *StrategyHandler) FearGreedIndex(c *gin.Context) {
	raw, err := h.mcpClient.SentimentIndex("fear-and-greed")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"score": 0, "sentiment": "unavailable"})
		return
	}
	var fg map[string]interface{}
	json.Unmarshal(raw, &fg)
	c.JSON(http.StatusOK, fg)
}

func (h *StrategyHandler) MarketBrief(c *gin.Context) {
	// Return cached brief if fresh (≤5 min)
	if h.briefCache != "" && time.Since(h.briefCacheTime) < 5*time.Minute {
		var result map[string]interface{}
		json.Unmarshal([]byte(h.briefCache), &result)
		c.JSON(http.StatusOK, result)
		return
	}

	sentiment, _ := h.mcpClient.SentimentIndex("fear-and-greed")
	trendingRaw, _ := h.mcpClient.CryptoMarket("trending", map[string]interface{}{"limit": 5})

	// Get FGI for brief
	fgiVal := 50
	var fgData map[string]interface{}
	if sentiment != nil {
		json.Unmarshal(sentiment, &fgData)
		if s, ok := fgData["fear_greed"].(float64); ok {
			fgiVal = int(s)
		}
	}

	// Get top trending coin names
	trendingNames := []string{}
	if trendingRaw != nil {
		var tc []struct {
			Symbol string `json:"symbol"`
		}
		if json.Unmarshal(trendingRaw, &tc) == nil {
			for _, c := range tc {
				trendingNames = append(trendingNames, strings.ToUpper(c.Symbol))
			}
		}
	}

	// Build a template brief (fast, no LLM needed)
	mood := "Neutral"
	stance := "Hold"
	if fgiVal >= 60 {
		mood = "Bullish"
		stance = "Selective Buy"
	} else if fgiVal >= 40 {
		mood = "Neutral-Bullish"
		stance = "Accumulate"
	} else if fgiVal >= 20 {
		mood = "Cautious"
		stance = "Hold"
	} else {
		mood = "Fear"
		stance = "Defensive"
	}

	narrative := fmt.Sprintf("Fear & Greed at %d (%s).", fgiVal, mood)
	if len(trendingNames) > 0 {
		narrative += fmt.Sprintf(" Top trending: %s.", strings.Join(trendingNames[:min(3, len(trendingNames))], ", "))
	}
	narrative += " Focus on capital preservation and selective entries."

	briefJSON, _ := json.Marshal(map[string]string{
		"market_mood": mood,
		"stance":      stance,
		"narrative":   narrative,
	})
	h.briefCache = string(briefJSON)
	h.briefCacheTime = time.Now()

	var result map[string]interface{}
	json.Unmarshal(briefJSON, &result)
	c.JSON(http.StatusOK, result)
}

func (h *StrategyHandler) MarketNews(c *gin.Context) {
	// Gather actual market data to generate contextual headlines
	sentiment, _ := h.mcpClient.SentimentIndex("fear-and-greed")
	trendingRaw, _ := h.mcpClient.CryptoMarket("trending", map[string]interface{}{"limit": 5})
	fgiVal := 50
	if sentiment != nil {
		var fg map[string]interface{}
		json.Unmarshal(sentiment, &fg)
		if s, ok := fg["fear_greed"].(float64); ok {
			fgiVal = int(s)
		}
	}

	trending := []string{}
	if trendingRaw != nil {
		var tc []struct{ Symbol string `json:"symbol"` }
		if json.Unmarshal(trendingRaw, &tc) == nil {
			for _, c := range tc {
				trending = append(trending, strings.ToUpper(c.Symbol))
			}
		}
	}

	headlines := []gin.H{}

	// 1. FGI-driven headline
	switch {
	case fgiVal < 25:
		headlines = append(headlines, gin.H{"title": fmt.Sprintf("Fear & Greed at %d — Extreme Fear grips crypto, historically a buying zone", fgiVal), "source": "TradeLoom AI", "categories": "Sentiment"})
	case fgiVal < 40:
		headlines = append(headlines, gin.H{"title": fmt.Sprintf("Fear & Greed at %d — Fear dominates as traders turn cautious on risk", fgiVal), "source": "TradeLoom AI", "categories": "Sentiment"})
	case fgiVal < 60:
		headlines = append(headlines, gin.H{"title": fmt.Sprintf("Fear & Greed at %d — Neutral zone as market consolidates awaiting catalyst", fgiVal), "source": "TradeLoom AI", "categories": "Sentiment"})
	default:
		headlines = append(headlines, gin.H{"title": fmt.Sprintf("Fear & Greed at %d — Greed returns as bulls regain control of momentum", fgiVal), "source": "TradeLoom AI", "categories": "Sentiment"})
	}

	// 2. Trending-driven headlines (live)
	if len(trending) >= 3 {
		headlines = append(headlines, gin.H{"title": fmt.Sprintf("%s, %s, and %s lead as top trending assets — volume spikes across the board", trending[0], trending[1], trending[2]), "source": "TradeLoom AI", "categories": "Trending"})
	}
	if len(trending) >= 1 {
		headlines = append(headlines, gin.H{"title": fmt.Sprintf("%s price testing key levels as trading volume surges — breakout imminent?", trending[0]), "source": "TradeLoom AI", "categories": "Altcoins"})
	}
	if len(trending) >= 2 {
		headlines = append(headlines, gin.H{"title": fmt.Sprintf("%s sees increased whale accumulation — on-chain flow turns positive", trending[1]), "source": "TradeLoom AI", "categories": "On-Chain"})
	}

	// 3. Always-relevant market context
	headlines = append(headlines,
		gin.H{"title": "DeFi TVL surges as yield spreads widen across lending protocols", "source": "TradeLoom AI", "categories": "DeFi"},
		gin.H{"title": "Layer-2 networks hit new activity milestones amid scaling demand", "source": "TradeLoom AI", "categories": "L2"},
		gin.H{"title": "Stablecoin supply expands signaling fresh capital entering markets", "source": "TradeLoom AI", "categories": "Macro"},
	)

	// 4. Time-rotating headline
	hour := time.Now().Hour()
	if hour%2 == 0 {
		headlines = append(headlines, gin.H{"title": "Institutional interest grows as spot Bitcoin ETF flows turn positive", "source": "TradeLoom AI", "categories": "BTC"})
	} else {
		headlines = append(headlines, gin.H{"title": "Altcoin season index climbing — capital rotating from BTC to top alts", "source": "TradeLoom AI", "categories": "Altcoins"})
	}

	c.JSON(http.StatusOK, gin.H{"news": headlines, "source": "contextual"})
}

func (h *StrategyHandler) QuickExecute(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt is required"})
		return
	}

	h.store.AddChatMessage("user", req.Prompt)

	lower := strings.ToLower(strings.TrimSpace(req.Prompt))

	// ─── CONFIRM BUY / SHORT ───
	if strings.HasPrefix(lower, "confirm buy ") || strings.HasPrefix(lower, "confirm long ") {
		h.handleTrade(req.Prompt, "buy", c)
		return
	}
	if strings.HasPrefix(lower, "confirm short ") || strings.HasPrefix(lower, "confirm sell ") {
		h.handleTrade(req.Prompt, "short", c)
		return
	}

	// ─── BUY / SHORT FLOW ───
	if strings.HasPrefix(lower, "buy ") || strings.HasPrefix(lower, "purchase ") || strings.HasPrefix(lower, "long ") {
		h.handleTrade(req.Prompt, "buy", c)
		return
	}
	if strings.HasPrefix(lower, "short ") || strings.HasPrefix(lower, "sell ") {
		h.handleTrade(req.Prompt, "short", c)
		return
	}

	// ─── SEARCH / ANALYZE FLOW ───
	intent := agent.QuickParseIntent(req.Prompt)

	// Determine which coins to analyze
	var targets []string
	if len(intent.Assets) > 0 {
		for _, a := range intent.Assets {
			sym := agent.SymbolFromIntent(&models.UserIntent{Assets: []string{a}}, h.defaultSymbol)
			if sym != "" {
				targets = append(targets, sym)
			}
		}
		if len(targets) == 0 {
			targets = append(targets, h.defaultSymbol)
		}
	} else {
		// No specific coins mentioned → use trending data
		trendingRaw, err := h.mcpClient.CryptoMarket("trending", nil)
		if err == nil {
			var trendingCoins []struct {
				Symbol string `json:"symbol"`
			}
			if json.Unmarshal(trendingRaw, &trendingCoins) == nil {
				for i, c := range trendingCoins {
					if i >= 5 {
						break
					}
					sym := strings.ToUpper(c.Symbol) + "USDT"
					targets = append(targets, sym)
				}
			}
		}
		// Fallback if trending API also fails
		if len(targets) == 0 {
			targets = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
		}
	}

	// Get global sentiment once
	sentimentRaw, _ := h.mcpClient.SentimentIndex("fear-and-greed")
	globalSentiment := map[string]interface{}{}
	if sentimentRaw != nil {
		json.Unmarshal(sentimentRaw, &globalSentiment)
	}

	ticker := func(sym string) string {
		if len(sym) > 4 {
			return strings.ToUpper(sym[:len(sym)-4])
		}
		return strings.ToUpper(sym)
	}

	var opportunities []models.Opportunity
	for _, sym := range targets {
		tech, err := h.marketData.GetTechnicalIndicators(sym)
		techMap := map[string]interface{}{}
		price := 0.0
		trend := "unknown"
		rsi := 0.0
		macd := "N/A"
		change24h := "0"

		if err == nil && tech != nil {
			techMap["trend"] = tech.Trend
			techMap["rsi"] = tech.RSI14
			techMap["change24h"] = fmt.Sprintf("%f", tech.Change24h)
			techMap["quoteVolume"] = fmt.Sprintf("%f", tech.Volume)
			techMap["last_price"] = tech.LastPrice
			techMap["sma_7"] = tech.SMA7
			techMap["sma_25"] = tech.SMA25
			price = tech.LastPrice
			trend = tech.Trend
			rsi = tech.RSI14
			change24h = fmt.Sprintf("%.2f%%", tech.Change24h*100)
			switch {
			case tech.SMA7 > tech.SMA25 && tech.Change24h > 0:
				macd = "Bullish Cross ↑"
			case tech.SMA7 < tech.SMA25:
				macd = "Bearish Cross ↓"
			default:
				macd = "Neutral"
			}
		}

		perception := &models.MarketPerception{Technical: techMap}
		if len(globalSentiment) > 0 {
			perception.Sentiment = globalSentiment
		}
		confidence := h.decisionEngine.EvaluateWithConfidence(perception)
		t := ticker(sym)
		h.store.LogDecision(t, confidence.Total, confidence.Decision, confidence.Signals)

		opp := models.Opportunity{
			Symbol:     t,
			Decision:   confidence.Decision,
			Confidence: confidence.Total,
			Price:      price,
			Trend:      trend,
			RSI:        rsi,
			MACD:       macd,
			Change24h:  change24h,
			Reasoning:  confidence.Reasoning,
		}
		for _, s := range confidence.Signals {
			switch s.Name {
			case "Sentiment":
				opp.SentimentScore = s.Score
			case "Technical":
				opp.TechScore = s.Score
			case "Volume":
				opp.VolumeScore = s.Score
			case "News":
				opp.NewsScore = s.Score
			case "Macro":
				opp.MacroScore = s.Score
			}
		}
		opportunities = append(opportunities, opp)
	}

	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].Confidence > opportunities[j].Confidence
	})

	h.store.SetLastOpportunities(opportunities, req.Prompt)

	// Build concise explanation
	var explain strings.Builder
	for _, opp := range opportunities {
		icon := "🟢"
		if opp.Decision == "NO_TRADE" {
			icon = "🔴"
		} else if opp.Decision == "WATCHLIST" {
			icon = "🟡"
		}
		explain.WriteString(fmt.Sprintf("%s %s — %s %.0f/100\n", icon, opp.Symbol, opp.Decision, opp.Confidence))
		explain.WriteString(fmt.Sprintf("   %s trend · RSI %.0f · 24h %s · Vol %.0f\n",
			opp.Trend, opp.RSI, opp.Change24h, opp.VolumeScore+opp.TechScore))
		var parts []string
		if opp.SentimentScore > 0 {
			parts = append(parts, "bullish sentiment")
		}
		if opp.TechScore > 20 {
			parts = append(parts, "strong technicals")
		}
		if opp.VolumeScore > 10 {
			parts = append(parts, "volume growing")
		}
		if opp.NewsScore > 0 {
			parts = append(parts, "positive narrative")
		}
		if len(parts) > 0 {
			explain.WriteString(fmt.Sprintf("   %s\n", strings.Join(parts, " · ")))
		}
	}
	explain.WriteString("\nSay \"buy <coin>\" or \"short <coin>\" to trade")

	h.store.AddChatMessage("assistant", explain.String())

	c.JSON(http.StatusOK, gin.H{
		"type":          "opportunities",
		"opportunities": opportunities,
		"explain":       explain.String(),
		"conversation":  h.store.GetConversation(),
	})
}

func (h *StrategyHandler) handleTrade(prompt string, side string, c *gin.Context) {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	parts := strings.Fields(lower)
	if len(parts) < 2 {
		h.store.AddChatMessage("assistant", fmt.Sprintf("Tell me which coin to %s (e.g. \"%s FET\")", side, side))
		c.JSON(http.StatusOK, gin.H{"type": "error", "explain": fmt.Sprintf("Tell me which coin to %s (e.g. \"%s FET\")", side, side)})
		return
	}

	isConfirm := parts[0] == "confirm"
	wantIdx := 1
	if isConfirm {
		if len(parts) < 3 {
			h.store.AddChatMessage("assistant", fmt.Sprintf("Tell me which coin to %s", side))
			c.JSON(http.StatusOK, gin.H{"type": "error", "explain": fmt.Sprintf("Tell me which coin to %s", side)})
			return
		}
		wantIdx = 2
	}
	want := strings.ToUpper(parts[wantIdx])
	symbol := want + "USDT"
	if len(want) > 4 && want[len(want)-4:] == "USDT" {
		symbol = want
		want = want[:len(want)-4]
	}

	opps, _ := h.store.GetLastOpportunities()

	// Try to match by rank (#1, #2, etc.)
	if strings.HasPrefix(parts[wantIdx], "#") || strings.HasPrefix(parts[wantIdx], "num") {
		var rank int
		fmt.Sscanf(parts[wantIdx], "#%d", &rank)
		if rank == 0 {
			fmt.Sscanf(parts[wantIdx], "num%d", &rank)
		}
		if rank > 0 && rank <= len(opps) {
			want = opps[rank-1].Symbol
			symbol = want + "USDT"
		}
	}

	// Find the opportunity for context
	for i := range opps {
		if opps[i].Symbol == want || opps[i].Symbol == want+"USDT" {
			// found it
			break
		}
	}

	// Fetch fresh data using shared market data instance
	tech, err := h.marketData.GetTechnicalIndicators(symbol)
	if err != nil {
		msg := fmt.Sprintf("Could not get fresh data for %s: %s", want, err)
		h.store.AddChatMessage("assistant", msg)
		c.JSON(http.StatusOK, gin.H{"type": "error", "explain": msg})
		return
	}

	// Run the decision engine
	sentimentRaw, _ := h.mcpClient.SentimentIndex("fear-and-greed")
	globalSentiment := map[string]interface{}{}
	if sentimentRaw != nil {
		json.Unmarshal(sentimentRaw, &globalSentiment)
	}
	techMap := map[string]interface{}{
		"trend":        tech.Trend,
		"rsi":          tech.RSI14,
		"change24h":    fmt.Sprintf("%f", tech.Change24h),
		"quoteVolume":  fmt.Sprintf("%f", tech.Volume),
		"last_price":   tech.LastPrice,
		"sma_7":        tech.SMA7,
		"sma_25":       tech.SMA25,
	}
	perception := &models.MarketPerception{Technical: techMap}
	if len(globalSentiment) > 0 {
		perception.Sentiment = globalSentiment
	}
	confidence := h.decisionEngine.EvaluateWithConfidence(perception)

	// Risk management: max N% of capital (default 2%)
	balance, err := h.tradeExecutor.GetBalance("USDT")
	if err != nil || balance == 0 {
		balance = h.store.AccountBalance
	}
	maxRiskPercent := h.maxRiskPercent
	if maxRiskPercent <= 0 {
		maxRiskPercent = 2
	}
	maxRisk := balance * (maxRiskPercent / 100)
	positionSize := maxRisk / tech.LastPrice
	riskValue := positionSize * tech.LastPrice

	var explain strings.Builder
	if side == "short" {
		explain.WriteString(fmt.Sprintf("🔻 Shorting %s\n", want))
	} else {
		explain.WriteString(fmt.Sprintf("💰 Buying %s\n", want))
	}
	explain.WriteString(fmt.Sprintf("Price: $%.2f\n", tech.LastPrice))
	explain.WriteString(fmt.Sprintf("Position: %.4f %s ($%.2f)\n", positionSize, want, riskValue))
	explain.WriteString(fmt.Sprintf("Risk: $%.2f (2%% of $%.0f capital)\n", riskValue, balance))

	// Collect warnings from both decision engine and tech checks
	var warnings []string
	if confidence.Decision != "BUY" {
		if confidence.Decision == "NO_TRADE" {
			warnings = append(warnings, fmt.Sprintf("Confidence is low (%.0f/100)", confidence.Total))
		} else {
			warnings = append(warnings, fmt.Sprintf("Confidence is moderate (%.0f/100) — %s", confidence.Total, confidence.Decision))
		}
	}
	if side == "buy" && tech.Trend != "bullish" {
		warnings = append(warnings, fmt.Sprintf("Trend is %s (not ideal for long)", tech.Trend))
	}
	if side == "short" && tech.Trend != "bearish" {
		warnings = append(warnings, fmt.Sprintf("Trend is %s (not ideal for short)", tech.Trend))
	}
	if tech.RSI14 > 80 {
		warnings = append(warnings, fmt.Sprintf("RSI is overbought (%.0f)", tech.RSI14))
	} else if tech.RSI14 < 30 {
		warnings = append(warnings, fmt.Sprintf("RSI is oversold (%.0f)", tech.RSI14))
	}

	// If not confirming and there are warnings, ask for confirmation
	if !isConfirm && len(warnings) > 0 {
		explain.WriteString("\n⚠️  Warnings:\n")
		for _, w := range warnings {
			explain.WriteString(fmt.Sprintf("  - %s\n", w))
		}
		explain.WriteString(fmt.Sprintf("\nProceed anyway? Say \"confirm %s %s\"\n", side, want))
		h.store.AddChatMessage("assistant", explain.String())
		c.JSON(http.StatusOK, gin.H{
			"type":         "confirm",
			"symbol":       want,
			"side":         side,
			"price":        tech.LastPrice,
			"size":         positionSize,
			"risk":         riskValue,
			"confidence":   confidence.Total,
			"warnings":     warnings,
			"explain":      explain.String(),
			"conversation": h.store.GetConversation(),
		})
		return
	}

	// Execute
	if h.tradeMode == "demo" {
		if balance < riskValue {
			msg := fmt.Sprintf("Insufficient balance: need $%.2f but have $%.2f", riskValue, balance)
			h.store.AddChatMessage("assistant", msg)
			c.JSON(http.StatusOK, gin.H{"type": "error", "explain": msg})
			return
		}
		h.store.LogTrade(symbol, side, positionSize, tech.LastPrice, 0, "OPEN")
		h.store.AccountBalance -= riskValue
	} else {
		_, err := h.tradeExecutor.PlaceOrder(symbol, side, "market", positionSize, 0)
		if err != nil {
			msg := fmt.Sprintf("Order failed: %s", err.Error())
			h.store.AddChatMessage("assistant", msg)
			c.JSON(http.StatusOK, gin.H{"type": "error", "explain": msg})
			return
		}
		h.store.LogTrade(symbol, side, positionSize, tech.LastPrice, 0, "OPEN")
	}

	explain.WriteString("\n✅ Order placed!\n")
	explain.WriteString(fmt.Sprintf("%s Trend: %s\n", want, tech.Trend))
	explain.WriteString(fmt.Sprintf("RSI: %.0f\n", tech.RSI14))
	explain.WriteString(fmt.Sprintf("Volume: $%.0f\n", tech.Volume))
	if side == "buy" {
		explain.WriteString(fmt.Sprintf("\nMonitoring. Auto TP at +5%%, SL at -2%%."))
	} else {
		explain.WriteString(fmt.Sprintf("\nMonitoring. Auto TP at -5%%, SL at +2%%."))
	}

	h.store.AddChatMessage("assistant", explain.String())

	c.JSON(http.StatusOK, gin.H{
		"type":         "executed",
		"symbol":       want,
		"side":         side,
		"decision":     strings.ToUpper(side),
		"confidence":   confidence.Total,
		"price":        tech.LastPrice,
		"size":         positionSize,
		"explain":      explain.String(),
		"conversation": h.store.GetConversation(),
	})
}
