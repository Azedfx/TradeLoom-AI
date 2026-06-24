package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"trade/config"
	"trade/internal/agent"
	"trade/internal/api"
	"trade/internal/bitget"
	"trade/internal/llm"
	"trade/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	llmClient := llm.NewLLMClient(cfg.LLMApiKey(), cfg.LLMBaseURL, cfg.LLMModel)
	strategyCompiler := &agent.StrategyCompiler{}
	marketData := bitget.NewMarketData()
	decisionEngine := agent.NewDecisionEngine(marketData, llmClient)
	tradeExecutor := &bitget.TradeExecutor{}
	mcpClient := bitget.NewMCPClient("https://hackathon.bitgetops.com/mcp")
	memStore := store.NewMemoryStore("trade_log.csv", "data/state.json", cfg.DefaultCapital)

	monitor := agent.NewMonitor(
		memStore,
		marketData,
		decisionEngine,
		tradeExecutor,
		mcpClient,
		llmClient,
		time.Duration(cfg.MonitorInterval)*time.Second,
		cfg.DefaultCapital,
		cfg.MaxRiskPercent,
		cfg.TakeProfitPct,
		cfg.StopLossPct,
		cfg.TradeMode,
	)
	monitor.Start()

	handler := api.NewStrategyHandler(
		llmClient, strategyCompiler, decisionEngine,
		tradeExecutor, mcpClient, marketData, memStore,
		cfg.TradeMode, cfg.DefaultSymbol,
		cfg.DefaultCapital, cfg.MaxRiskPercent, cfg.TakeProfitPct, cfg.StopLossPct,
	)

	port := cfg.Port
	if port[0] != ':' {
		port = ":" + port
	}

	r := gin.Default()
	r.LoadHTMLGlob("templates/*.html")
	api.RegisterRoutes(r, handler)

	log.Printf("server starting on %s (%s mode)", port, cfg.TradeMode)
	log.Printf("dashboard → http://localhost%s", port)
	r.Run(port)
}
