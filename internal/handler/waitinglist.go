package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/service"
	"gorm.io/gorm"
)

type WaitingListHandler struct{ db *gorm.DB }

func NewWaitingListHandler(db *gorm.DB) *WaitingListHandler {
	return &WaitingListHandler{db: db}
}

// GetForCatalog retourne la liste d'attente d'un catalogue.
// GET /api/catalogs/:id/waiting-list
func (h *WaitingListHandler) GetForCatalog(c *gin.Context) {
	catalogID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid catalog id"})
		return
	}

	svc := service.NewWaitingListService(h.db)
	list, err := svc.GetForCatalog(uint(catalogID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]gin.H, 0, len(list))
	for _, e := range list {
		out = append(out, gin.H{
			"id":        e.ID,
			"userId":    e.UserID,
			"userName":  e.User.Name(),
			"catalogId": e.CatalogID,
			"message":   e.Message,
			"date":      e.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "waitingList": out})
}

// Join inscrit l'utilisateur connecté sur la liste d'attente.
// POST /api/catalogs/:id/waiting-list
func (h *WaitingListHandler) Join(c *gin.Context) {
	claims := middleware.GetClaims(c)

	catalogID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid catalog id"})
		return
	}

	var payload struct {
		Message *string `json:"message"`
	}
	c.ShouldBindJSON(&payload)

	svc := service.NewWaitingListService(h.db)
	entry, err := svc.Join(claims.UserID, uint(catalogID), payload.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"position": svc.Position(claims.UserID, uint(catalogID)),
		"id":       entry.ID,
	})
}

// Leave retire l'utilisateur connecté de la liste d'attente.
// DELETE /api/catalogs/:id/waiting-list
func (h *WaitingListHandler) Leave(c *gin.Context) {
	claims := middleware.GetClaims(c)

	catalogID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid catalog id"})
		return
	}

	svc := service.NewWaitingListService(h.db)
	if err := svc.Leave(claims.UserID, uint(catalogID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
