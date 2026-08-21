package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AdminOrderLine : une ligne de commande, avec de quoi la regrouper des deux
// facons dont on s en sert — par produit pour preparer avec les producteurs,
// par adherent pour distribuer. Le regroupement se fait cote interface : deux
// requetes pour les memes lignes seraient du gaspillage.
type AdminOrderLine struct {
	UserID     uint    `json:"userId"`
	UserName   string  `json:"userName"`
	VendorID   uint    `json:"vendorId"`
	VendorName string  `json:"vendorName"`
	ProductRef string  `json:"productRef,omitempty"`
	Product    string  `json:"product"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unitPrice"`
	Total      float64 `json:"total"`
	// NeedsWeighing : le prix se fixe a la pesee. C est une particularite du
	// metier qu aucun logiciel de vente en ligne ne connait, et le tableau
	// serait faux sans elle.
	NeedsWeighing bool `json:"needsWeighing"`
	Weighed       bool `json:"weighed"`
}

// AdminDistributionOrders : GET /api/admin/distributions/:id/orders.
func (h *PagesHandler) AdminDistributionOrders(c *gin.Context) {
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

	// L appartenance au groupe est verifiee sur la distribution elle-meme : un
	// identifiant tape dans l URL ouvrirait sinon les commandes d un autre
	// groupe.
	var md model.MultiDistrib
	if err := h.db.Where("id = ? AND group_id = ?", id, claims.GroupID).First(&md).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "distribution introuvable"})
		return
	}

	var commandes []model.UserOrder
	h.db.
		Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Where("distributions.multi_distrib_id = ?", md.ID).
		Preload("User").
		Preload("Product").
		Preload("Product.Catalog.Vendor").
		Order("user_orders.id").
		Find(&commandes)

	lignes := make([]AdminOrderLine, 0, len(commandes))
	var total float64
	for _, o := range commandes {
		ref := ""
		if o.Product.Ref != nil {
			ref = *o.Product.Ref
		}
		ligne := AdminOrderLine{
			UserID:        o.UserID,
			UserName:      o.User.Name(),
			VendorID:      o.Product.Catalog.Vendor.ID,
			VendorName:    o.Product.Catalog.Vendor.Name,
			ProductRef:    ref,
			Product:       o.Product.Name,
			Quantity:      o.Quantity,
			UnitPrice:     o.ProductPrice,
			Total:         o.TotalPrice(),
			NeedsWeighing: o.Product.MultiWeight || o.Product.VariablePrice,
			Weighed:       o.ForcedPrice != nil,
		}
		total += ligne.Total
		lignes = append(lignes, ligne)
	}

	c.JSON(http.StatusOK, gin.H{"lines": lignes, "total": total})
}
