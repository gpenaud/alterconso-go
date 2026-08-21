package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AdminCatalogProduct : une ligne du catalogue, vue par qui le tient a jour.
type AdminCatalogProduct struct {
	ID     uint    `json:"id"`
	Ref    string  `json:"ref,omitempty"`
	Name   string  `json:"name"`
	Unit   string  `json:"unit,omitempty"`
	Price  float64 `json:"price"`
	Active bool    `json:"active"`
	// Un produit retire reste affiche, en retrait : il n est pas supprime, et
	// le voir disparaitre ferait croire a une perte.
	NeedsWeighing bool     `json:"needsWeighing"`
	StockTracked  bool     `json:"stockTracked"`
	Stock         *float64 `json:"stock,omitempty"`
	Organic       bool     `json:"organic"`
}

// AdminCatalogProducts : GET /api/admin/catalogs/:id/products.
func (h *PagesHandler) AdminCatalogProducts(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil || claims.GroupID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun groupe courant"})
		return
	}
	ug := groupAccess(c, h.db, claims.UserID, claims.GroupID)
	if ug == nil || !(ug.IsGroupManager() || ug.HasRight(model.RightCatalogAdmin)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "acces refuse"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifiant invalide"})
		return
	}

	// Le catalogue doit appartenir au groupe courant : la verification porte
	// sur la requete elle-meme, et non sur ce que l interface a bien voulu
	// demander.
	var catalogue model.Catalog
	if err := h.db.Preload("Vendor").
		Where("id = ? AND group_id = ?", id, claims.GroupID).
		First(&catalogue).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "catalogue introuvable"})
		return
	}

	// Un responsable de catalogue n administre que les siens.
	if !ug.IsGroupManager() && !ug.HasRight(model.RightCatalogAdmin, strconv.FormatUint(id, 10)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "ce catalogue ne vous est pas confie"})
		return
	}

	var produits []model.Product
	h.db.Where("catalog_id = ?", catalogue.ID).Order("name").Find(&produits)

	vues := make([]AdminCatalogProduct, 0, len(produits))
	for _, p := range produits {
		ref := ""
		if p.Ref != nil {
			ref = *p.Ref
		}
		vues = append(vues, AdminCatalogProduct{
			ID:            p.ID,
			Ref:           ref,
			Name:          p.Name,
			Unit:          uniteLisible(p),
			Price:         p.Price,
			Active:        p.Active,
			NeedsWeighing: p.MultiWeight || p.VariablePrice,
			StockTracked:  p.StockTracked,
			Stock:         p.Stock,
			Organic:       p.Organic,
		})
	}

	nomProducteur := ""
	if catalogue.Vendor.ID != 0 {
		nomProducteur = catalogue.Vendor.Name
	}

	c.JSON(http.StatusOK, gin.H{
		"catalogId":  catalogue.ID,
		"name":       catalogue.Name,
		"vendorName": nomProducteur,
		"products":   vues,
	})
}
