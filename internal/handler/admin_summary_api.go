package handler

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// DistributionSummary : l etat d une distribution, vu par qui l organise.
//
// Une seule requete pour ce qu il faut savoir avant un jeudi : combien ont
// commande, pour quel montant, ce qui manque encore, et ce qu il faudra etaler
// sur les tables.
type DistributionSummary struct {
	MultiDistribID  uint             `json:"multiDistribId"`
	NbOrders        int              `json:"nbOrders"`
	NbMembers       int              `json:"nbMembers"`
	Total           float64          `json:"total"`
	AverageOrder    float64          `json:"averageOrder"`
	NbVendors       int              `json:"nbVendors"`
	VolunteerNeeded int              `json:"volunteerNeeded"`
	TopProducts     []TopProductView `json:"topProducts"`
}

type TopProductView struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
}

// AdminDistributionSummary : GET /api/admin/distributions/:id/summary.
//
// Reserve a qui administre le groupe : l ecran expose des montants et des
// effectifs qui ne regardent pas un adherent.
func (h *PagesHandler) AdminDistributionSummary(c *gin.Context) {
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

	var md model.MultiDistrib
	if err := h.db.Preload("Distributions.Catalog").
		Where("id = ? AND group_id = ?", id, claims.GroupID).
		First(&md).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "distribution introuvable"})
		return
	}

	var commandes []model.UserOrder
	h.db.
		Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Where("distributions.multi_distrib_id = ?", md.ID).
		Preload("Product").
		Find(&commandes)

	resume := DistributionSummary{MultiDistribID: md.ID}

	// Le nombre de commandes se compte en adherents, pas en lignes : un panier
	// de six produits reste une commande.
	adherents := map[uint]bool{}
	quantiteParProduit := map[string]float64{}
	for _, o := range commandes {
		adherents[o.UserID] = true
		resume.Total += o.TotalPrice()
		quantiteParProduit[o.Product.Name] += o.Quantity
	}
	resume.NbOrders = len(adherents)
	if resume.NbOrders > 0 {
		resume.AverageOrder = resume.Total / float64(resume.NbOrders)
	}

	vendeurs := map[uint]bool{}
	for _, d := range md.Distributions {
		if d.Catalog.VendorID != 0 {
			vendeurs[d.Catalog.VendorID] = true
		}
	}
	resume.NbVendors = len(vendeurs)

	var nbMembres int64
	h.db.Model(&model.UserGroup{}).Where("group_id = ?", claims.GroupID).Count(&nbMembres)
	resume.NbMembers = int(nbMembres)

	var roles []model.VolunteerRole
	h.db.Where("group_id = ?", claims.GroupID).Find(&roles)
	var inscrits int64
	h.db.Model(&model.Volunteer{}).Where("multi_distrib_id = ?", md.ID).Count(&inscrits)
	if manque := len(roles) - int(inscrits); manque > 0 {
		resume.VolunteerNeeded = manque
	}

	for nom, quantite := range quantiteParProduit {
		resume.TopProducts = append(resume.TopProducts, TopProductView{Name: nom, Quantity: quantite})
	}
	sort.Slice(resume.TopProducts, func(i, j int) bool {
		return resume.TopProducts[i].Quantity > resume.TopProducts[j].Quantity
	})
	if len(resume.TopProducts) > 5 {
		resume.TopProducts = resume.TopProducts[:5]
	}

	c.JSON(http.StatusOK, resume)
}
