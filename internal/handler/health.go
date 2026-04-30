package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Liveness retourne 200 tant que le process Go répond. Aucune dépendance
// externe n'est testée : un échec de cette probe = process bloqué / OOM,
// k8s kill et redémarre. Tester la DB ici cascadrait un blip MySQL en
// kill de tous les pods, ce qu'on ne veut surtout pas.
func Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// Readiness vérifie que le pod peut effectivement servir du trafic :
// le ping DB doit passer dans un délai raisonnable. Échec → k8s retire
// le pod du Service le temps que la dépendance redevienne saine, sans
// le tuer.
//
// Le timeout court (1 s) évite qu'une DB lente ne bloque le worker
// gin pendant toute la durée par défaut du driver MySQL (~30 s).
func Readiness(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()

		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"db":     "unavailable",
				"error":  err.Error(),
			})
			return
		}
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"db":     "unreachable",
				"error":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "ok"})
	}
}
