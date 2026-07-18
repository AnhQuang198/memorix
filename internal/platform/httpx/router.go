package httpx

import "github.com/gin-gonic/gin"

// NewRouter dựng Gin engine với route nền tảng (/api/v1). Module handler
// đăng ký thêm route qua RegisterModule ở cmd/api.
func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	return r
}
