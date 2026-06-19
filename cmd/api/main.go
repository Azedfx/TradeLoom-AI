package main

import (
	"log"

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
	marketData := &bitget.MarketData{}
	decisionEngine := agent.NewDecisionEngine(marketData)
	tradeExecutor := &bitget.TradeExecutor{}
	mcpClient := bitget.NewMCPClient("https://hackathon.bitgetops.com/mcp")
	memStore := store.NewMemoryStore()

	monitor := agent.NewMonitor(memStore, marketData, decisionEngine, tradeExecutor, mcpClient, llmClient)
	monitor.Start()

	handler := api.NewStrategyHandler(
		llmClient, strategyCompiler, decisionEngine,
		tradeExecutor, mcpClient, memStore, cfg.TradeMode, cfg.DefaultSymbol,
	)

	r := gin.Default()
	api.RegisterRoutes(r, handler)

	log.Printf("server starting on %s (%s mode)", cfg.Port, cfg.TradeMode)
	r.Run(cfg.Port)
}
