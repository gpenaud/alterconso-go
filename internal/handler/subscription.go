package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/service"
	"gorm.io/gorm"
)

type SubscriptionHandler struct{ db *gorm.DB }

func NewSubscriptionHandler(db *gorm.DB) *SubscriptionHandler {
	return &SubscriptionHandler{db: db}
}

// GetForCatalog retourne les abonnements de l'utilisateur connecté pour un catalogue.
// GET /api/catalogs/:id/subscriptions
func (h *SubscriptionHandler) GetForCatalog(c *gin.Context) {
	claims := middleware.GetClaims(c)

	catalogID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid catalog id"})
		return
	}

	svc := service.NewSubscriptionService(h.db)
	subs, err := svc.GetForUser(claims.UserID, uint(catalogID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]gin.H, 0, len(subs))
	for _, s := range subs {
		out = append(out, gin.H{
			"id":         s.ID,
			"catalogId":  s.CatalogID,
			"startDate":  s.StartDate,
			"endDate":    s.EndDate,
			"quantities": svc.GetQuantities(&s),
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "subscriptions": out})
}

// Subscribe crée ou met à jour un abonnement.
// POST /api/catalogs/:id/subscriptions
func (h *SubscriptionHandler) Subscribe(c *gin.Context) {
	claims := middleware.GetClaims(c)

	catalogID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid catalog id"})
		return
	}

	var payload struct {
		Quantities map[uint]float64 `json:"quantities" binding:"required"`
		StartDate  *time.Time       `json:"startDate"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startDate := time.Now()
	if payload.StartDate != nil {
		startDate = *payload.StartDate
	}

	svc := service.NewSubscriptionService(h.db)
	sub, err := svc.Subscribe(claims.UserID, uint(catalogID), service.QuantityMap(payload.Quantities), startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"subscription": gin.H{
			"id":         sub.ID,
			"catalogId":  sub.CatalogID,
			"startDate":  sub.StartDate,
			"endDate":    sub.EndDate,
			"quantities": svc.GetQuantities(sub),
		},
	})
}

// Unsubscribe clôture un abonnement.
// DELETE /api/subscriptions/:id
func (h *SubscriptionHandler) Unsubscribe(c *gin.Context) {
	claims := middleware.GetClaims(c)

	subID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}

	svc := service.NewSubscriptionService(h.db)
	if err := svc.Unsubscribe(uint(subID), claims.UserID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
