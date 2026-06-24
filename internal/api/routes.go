package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, h *StrategyHandler) {
	r.GET("/", h.Dashboard)
	r.POST("/strategies", h.CreateStrategy)
	r.POST("/strategies/:id/execute", h.ExecuteStrategy)
	r.GET("/strategies/:id/status", h.GetStrategyStatus)
	r.GET("/trades", h.ListTrades)
	r.GET("/trades/export", h.ExportTradesCSV)
	r.GET("/trending", h.TrendingCoins)
	r.GET("/fear-greed", h.FearGreedIndex)
	r.GET("/brief", h.MarketBrief)
	r.GET("/news", h.MarketNews)
	r.POST("/quick-execute", h.QuickExecute)
}
