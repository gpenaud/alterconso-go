package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// VendorDetailView : la fiche d un producteur telle qu un adherent la lit.
//
// Elle porte ce qui donne envie d ouvrir son catalogue — d ou il vient, ce
// qu il cultive, depuis quand il livre le groupe — et non les champs de
// gestion, qui ne le regardent pas.
type VendorDetailView struct {
	ID          uint                `json:"id"`
	Name        string              `json:"name"`
	City        string              `json:"city,omitempty"`
	Description string              `json:"description,omitempty"`
	Organic     bool                `json:"organic"`
	NbProducts  int                 `json:"nbProducts"`
	Products    []VendorProductView `json:"products"`
}

type VendorProductView struct {
	ID      uint    `json:"id"`
	Name    string  `json:"name"`
	Price   float64 `json:"price"`
	Unit    string  `json:"unit,omitempty"`
	Organic bool    `json:"organic"`
}

// VendorDetail : GET /api/vendors/:id — la fiche publique d un producteur,
// restreinte au groupe courant.
//
// La restriction n est pas cosmetique : un identifiant tape dans l URL
// donnerait sinon acces au catalogue d un producteur d un autre groupe.
func (h *CompatHandler) VendorDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifiant invalide"})
		return
	}

	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "non authentifie"})
		return
	}
	groupID := claims.GroupID
	if groupID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun groupe courant"})
		return
	}

	// Le producteur doit servir CE groupe : la jointure sur les catalogues
	// porte cette condition.
	var vendor model.Vendor
	if err := h.db.
		Joins("JOIN catalogs ON catalogs.vendor_id = vendors.id").
		Where("vendors.id = ? AND catalogs.group_id = ?", id, groupID).
		Group("vendors.id").
		First(&vendor).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "producteur introuvable"})
		return
	}

	var produits []model.Product
	h.db.
		Joins("JOIN catalogs ON catalogs.id = products.catalog_id").
		Where("catalogs.vendor_id = ? AND catalogs.group_id = ? AND products.active = ?", vendor.ID, groupID, true).
		Order("products.name").
		Find(&produits)

	vue := VendorDetailView{
		ID:         vendor.ID,
		Name:       vendor.Name,
		Organic:    vendor.Organic,
		NbProducts: len(produits),
		Products:   make([]VendorProductView, 0, len(produits)),
	}
	if vendor.City != nil {
		vue.City = *vendor.City
	}
	if vendor.Description != nil {
		vue.Description = *vendor.Description
	}
	for _, p := range produits {
		vue.Products = append(vue.Products, VendorProductView{
			ID:      p.ID,
			Name:    p.Name,
			Price:   p.Price,
			Unit:    uniteLisible(p),
			Organic: p.Organic,
		})
	}

	c.JSON(http.StatusOK, vue)
}

// uniteLisible met le conditionnement en mots : « le kilo », « env. 500 g ».
// Le type d unite seul — « Kilogram » — ne se montre pas a un adherent.
func uniteLisible(p model.Product) string {
	unites := map[model.UnitType]string{
		model.UnitTypeKilogram:   "kg",
		model.UnitTypeGram:       "g",
		model.UnitTypeLitre:      "L",
		model.UnitTypeCentilitre: "cl",
		model.UnitTypeMillilitre: "ml",
	}
	unite, connue := unites[p.UnitType]
	if !connue {
		return "la pièce"
	}
	if p.Qt != nil && *p.Qt > 0 {
		return "env. " + strconv.FormatFloat(*p.Qt, 'g', -1, 64) + " " + unite
	}
	return "le " + unite
}
