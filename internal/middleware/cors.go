package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
)

// CORS autorise les requêtes cross-origin en mode debug.
func CORS(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Debug {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
		}
		c.Next()
	}
}
