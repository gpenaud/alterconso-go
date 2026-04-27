package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"github.com/gpenaud/alterconso/internal/service"
	"gorm.io/gorm"
)

type VolunteerHandler struct{ db *gorm.DB }

func NewVolunteerHandler(db *gorm.DB) *VolunteerHandler {
	return &VolunteerHandler{db: db}
}

// GetForDistrib retourne les bénévoles pour un MultiDistrib.
// GET /api/distributions/:id/volunteers
func (h *VolunteerHandler) GetForDistrib(c *gin.Context) {
	distribID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid distribution id"})
		return
	}

	svc := service.NewVolunteerService(h.db)
	volunteers, err := svc.GetForDistrib(uint(distribID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]gin.H, 0, len(volunteers))
	for _, v := range volunteers {
		out = append(out, gin.H{
			"id":     v.ID,
			"userId": v.UserID,
			"name":   v.User.Name(),
			"role":   v.Role,
			"date":   v.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "volunteers": out})
}

// Register inscrit l'utilisateur connecté comme bénévole.
// POST /api/distributions/:id/volunteers
func (h *VolunteerHandler) Register(c *gin.Context) {
	claims := middleware.GetClaims(c)

	distribID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid distribution id"})
		return
	}

	var payload struct {
		Role   *string `json:"role"`
		UserID *uint   `json:"userId"` // admin peut inscrire quelqu'un d'autre
	}
	c.ShouldBindJSON(&payload)

	targetID := claims.UserID
	if payload.UserID != nil && *payload.UserID != claims.UserID {
		// Vérifier que le demandeur est admin du groupe
		var md model.MultiDistrib
		if err := h.db.First(&md, distribID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "distribution not found"})
			return
		}
		ug := loadGroupAccess(h.db, claims.UserID, md.GroupID)
		if ug == nil || !ug.IsGroupManager() {
			c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can register volunteers for others"})
			return
		}
		targetID = *payload.UserID
	}

	svc := service.NewVolunteerService(h.db)
	v, err := svc.Register(targetID, uint(distribID), payload.Role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "id": v.ID})
}

// Unregister désinscrit un bénévole.
// DELETE /api/volunteers/:id
func (h *VolunteerHandler) Unregister(c *gin.Context) {
	claims := middleware.GetClaims(c)

	vID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid volunteer id"})
		return
	}

	svc := service.NewVolunteerService(h.db)
	if err := svc.Unregister(uint(vID), claims.UserID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
