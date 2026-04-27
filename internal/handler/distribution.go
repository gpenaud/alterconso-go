package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type DistributionHandler struct{ db *gorm.DB }

func NewDistributionHandler(db *gorm.DB) *DistributionHandler { return &DistributionHandler{db: db} }

// List retourne les prochaines distributions d'un groupe.
func (h *DistributionHandler) List(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	claims := middleware.GetClaims(c)
	if loadGroupAccess(h.db, claims.UserID, uint(groupID)) == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
		return
	}

	// Récupérer les MultiDistribs à venir pour ce groupe
	var multiDistribs []model.MultiDistrib
	h.db.
		Where("group_id = ? AND distrib_start_date >= ?", groupID, time.Now()).
		Preload("Place").
		Preload("Distributions.Catalog.Vendor").
		Order("distrib_start_date").
		Limit(20).
		Find(&multiDistribs)

	c.JSON(http.StatusOK, multiDistribs)
}

// Get retourne le détail d'une distribution avec ses commandes.
func (h *DistributionHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var distrib model.Distribution
	if err := h.db.
		Preload("Catalog.Vendor").
		Preload("MultiDistrib.Place").
		First(&distrib, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "distribution not found"})
		return
	}

	// Vérifier que le demandeur est membre du groupe (ou admin site-wide)
	claims := middleware.GetClaims(c)
	ug := loadGroupAccess(h.db, claims.UserID, distrib.Catalog.GroupID)
	if ug == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
		return
	}

	// Charger les commandes si admin ou gestionnaire
	var orders []model.UserOrder
	if ug.IsGroupManager() {
		h.db.
			Where("distribution_id = ?", distrib.ID).
			Preload("User").
			Preload("Product").
			Find(&orders)
	}

	c.JSON(http.StatusOK, gin.H{
		"distribution": distrib,
		"canOrder":     distrib.CanOrderNow(),
		"orders":       orders,
	})
}
