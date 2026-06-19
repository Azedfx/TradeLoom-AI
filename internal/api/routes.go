package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, h *StrategyHandler) {
	r.GET("/", h.Dashboard)

	api := r.Group("/api/v1")
	api.POST("/strategies", h.CreateStrategy)
	api.POST("/strategies/:id/execute", h.ExecuteStrategy)
	api.GET("/strategies/:id/status", h.GetStrategyStatus)
}
