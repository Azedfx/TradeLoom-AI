package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type StrategyHandler struct {
	intentParser     *llm.LLMClient
	strategyCompiler *agent.StrategyCompiler
	decisionEngine   *agent.DecisionEngine
	tradeExecutor    *bitget.TradeExecutor
	mcpClient        *bitget.MCPClient
	store            *store.MemoryStore
	tradeMode        string
	defaultSymbol    string
}

func NewStrategyHandler(
	llm *llm.LLMClient,
	sc *agent.StrategyCompiler,
	de *agent.DecisionEngine,
	te *bitget.TradeExecutor,
	mcp *bitget.MCPClient,
	s *store.MemoryStore,
	tradeMode string,
	defaultSymbol string,
) *StrategyHandler {
	return &StrategyHandler{
		intentParser:     llm,
		strategyCompiler: sc,
		decisionEngine:   de,
		tradeExecutor:    te,
		mcpClient:        mcp,
		store:            s,
		tradeMode:        tradeMode,
		defaultSymbol:    defaultSymbol,
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

	marketData := &bitget.MarketData{}

	technical, err := marketData.GetTechnicalIndicators(symbol)
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
		capital = 1000 // Fallback for demo
	}
	maxRisk := capital * 0.02

	lastPrice, ok := techMap["last_price"].(float64)
	if !ok {
		lastPrice = 0
	}

	size := 0.0
	riskMgmt := gin.H{
		"capital":  capital,
		"max_risk": maxRisk,
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
	c.JSON(http.StatusOK, gin.H{
		"strategies":  strategies,
		"trades":      trades,
		"decisions":   decisions,
		"totalPnL":    h.store.CalculatePnL(),
		"winRate":     h.store.CalculateWinRate(),
		"mode":        h.tradeMode,
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
	ticker := h.defaultSymbol
	if t := c.Query("symbol"); t != "" {
		ticker = t
	}

	sentiment, _ := h.mcpClient.SentimentIndex("fear-and-greed")
	news, _ := h.mcpClient.NewsFeed("cryptopanic", ticker, 5)
	macro, _ := h.mcpClient.MacroIndicators("latest", "cpi")
	trending, _ := h.mcpClient.CryptoMarket("trending", map[string]interface{}{"limit": 5})

	brief, err := h.intentParser.GenerateMarketBrief(ticker,
		string(sentiment), string(news), string(macro), string(trending))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error(), "market_mood": "Unavailable"})
		return
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(brief), &result)
	c.JSON(http.StatusOK, result)
}

func (h *StrategyHandler) QuickExecute(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt is required"})
		return
	}

	intent, err := h.intentParser.ParseIntent(req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rules := h.strategyCompiler.Compile(intent)
	symbol := agent.ResolveSymbol(rules.AssetFilter, h.defaultSymbol)
	ticker := symbol
	if len(symbol) > 4 {
		ticker = symbol[:len(symbol)-4]
	}
	ticker = strings.ToUpper(ticker)

	strategy := &models.Strategy{
		ID:           uuid.New().String(),
		UserPrompt:   req.Prompt,
		Rules:        *rules,
		TargetSymbol: symbol,
		Active:       true,
		CreatedAt:    time.Now(),
	}
	h.store.SaveStrategy(strategy)

	// Immediately execute
	marketData := &bitget.MarketData{}
	technical, err := marketData.GetTechnicalIndicators(symbol)
	techMap := map[string]interface{}{}
	if err == nil && technical != nil {
		techMap["trend"] = technical.Trend
		techMap["rsi"] = technical.RSI14
		techMap["change24h"] = fmt.Sprintf("%f", technical.Change24h)
		techMap["quoteVolume"] = fmt.Sprintf("%f", technical.Volume)
		techMap["last_price"] = technical.LastPrice
		techMap["sma_7"] = technical.SMA7
		techMap["sma_25"] = technical.SMA25
	}

	sentiment, _ := h.mcpClient.SentimentIndex("fear-and-greed")
	news, _ := h.mcpClient.NewsFeed("cryptopanic", ticker, 5)
	macro, _ := h.mcpClient.MacroIndicators("latest", "cpi")
	marketAnalysis, _ := h.intentParser.AnalyzeMarketWithData(symbol, string(sentiment), string(news), string(macro))

	perception := &models.MarketPerception{
		Technical: techMap,
	}
	if marketAnalysis != nil {
		perception.Sentiment = marketAnalysis.Sentiment
		perception.News = marketAnalysis.News
		perception.Macro = marketAnalysis.Macro
	}

	confidence := h.decisionEngine.EvaluateWithConfidence(perception)
	h.store.LogDecision(ticker, confidence.Total, confidence.Decision, confidence.Signals)
	explain := h.decisionEngine.Explain(ticker, confidence)

	c.JSON(http.StatusOK, gin.H{
		"symbol":           symbol,
		"decision":         confidence.Decision,
		"confidence_score": confidence,
		"explain":          explain,
	})
}
