package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, h *StrategyHandler) {
	api := r.Group("/api/v1")
	api.GET("/dashboard", h.Dashboard)
	api.POST("/strategies", h.CreateStrategy)
	api.POST("/strategies/:id/execute", h.ExecuteStrategy)
	api.GET("/strategies/:id/status", h.GetStrategyStatus)
	api.GET("/trades", h.ListTrades)
	api.GET("/trades/export", h.ExportTradesCSV)
	api.GET("/trending", h.TrendingCoins)
	api.GET("/fear-greed", h.FearGreedIndex)
	api.GET("/brief", h.MarketBrief)
	api.POST("/quick-execute", h.QuickExecute)
}
