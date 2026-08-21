package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AdminMarkDelivered : POST /api/admin/distributions/:id/delivery.
//
// Marque le panier d un adherent comme remis, ou revient dessus. L operation
// porte sur toutes ses lignes d un coup : on remet un panier, pas un produit.
func (h *PagesHandler) AdminMarkDelivered(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil || claims.GroupID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun groupe courant"})
		return
	}
	ug := groupAccess(c, h.db, claims.UserID, claims.GroupID)
	if ug == nil || !(ug.IsGroupManager() || ug.CanManageDistributions()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "acces refuse"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifiant invalide"})
		return
	}

	var payload struct {
		UserID    uint `json:"userId"`
		Delivered bool `json:"delivered"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || payload.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "requete illisible"})
		return
	}

	var md model.MultiDistrib
	if err := h.db.Where("id = ? AND group_id = ?", id, claims.GroupID).First(&md).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "distribution introuvable"})
		return
	}

	var commandes []model.UserOrder
	h.db.
		Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Where("distributions.multi_distrib_id = ? AND user_orders.user_id = ?", md.ID, payload.UserID).
		Find(&commandes)
	if len(commandes) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "aucune commande pour cet adherent"})
		return
	}

	for _, o := range commandes {
		drapeaux := o.Flags
		if payload.Delivered {
			drapeaux |= uint(model.OrderFlagDelivered)
		} else {
			drapeaux &^= uint(model.OrderFlagDelivered)
		}
		// Mise a jour ciblee sur la colonne : un Save reecrirait l utilisateur
		// charge par Preload, dont la date de naissance vaut « 0000-00-00 »
		// pour les comptes issus de l import — que MySQL refuse en mode strict.
		h.db.Model(&model.UserOrder{}).Where("id = ?", o.ID).Update("flags", drapeaux)
	}

	c.JSON(http.StatusOK, gin.H{"userId": payload.UserID, "delivered": payload.Delivered})
}
