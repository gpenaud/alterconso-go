package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type CatalogHandler struct{ db *gorm.DB }

func NewCatalogHandler(db *gorm.DB) *CatalogHandler { return &CatalogHandler{db: db} }

// List retourne les catalogues d'un groupe (actifs par défaut, tous si ?all=true).
func (h *CatalogHandler) List(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	claims := middleware.GetClaims(c)
	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, groupID).First(&ug).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
		return
	}

	q := h.db.Where("group_id = ?", groupID).Preload("Vendor")

	// Filtre actifs uniquement sauf si ?all=true
	if c.Query("all") != "true" {
		q = q.Where("(end_date IS NULL OR end_date >= NOW()) AND (start_date IS NULL OR start_date <= NOW())")
	}

	var catalogs []model.Catalog
	q.Order("name").Find(&catalogs)

	c.JSON(http.StatusOK, catalogs)
}

func (h *CatalogHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var catalog model.Catalog
	if err := h.db.Preload("Vendor").Preload("Contact").First(&catalog, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "catalog not found"})
		return
	}

	// Vérifier que le demandeur est membre du groupe
	claims := middleware.GetClaims(c)
	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, catalog.GroupID).First(&ug).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
		return
	}

	// Charger les produits
	var products []model.Product
	h.db.Where("catalog_id = ?", catalog.ID).Order("name").Find(&products)

	c.JSON(http.StatusOK, gin.H{"catalog": catalog, "products": products})
}

func (h *CatalogHandler) Create(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	claims := middleware.GetClaims(c)
	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, groupID).First(&ug).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}
	if !ug.IsGroupManager() {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can create catalogs"})
		return
	}

	var payload struct {
		Name     string            `json:"name"     binding:"required"`
		Type     model.CatalogType `json:"type"`
		VendorID uint              `json:"vendorId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Vérifier que le vendor existe
	var vendor model.Vendor
	if err := h.db.First(&vendor, payload.VendorID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vendor not found"})
		return
	}

	catalog := model.Catalog{
		Name:     payload.Name,
		Type:     payload.Type,
		GroupID:  uint(groupID),
		VendorID: payload.VendorID,
	}
	catalog.SetFlag(model.CatalogFlagUsersCanOrder)

	if err := h.db.Create(&catalog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, catalog)
}

func (h *CatalogHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var catalog model.Catalog
	if err := h.db.First(&catalog, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "catalog not found"})
		return
	}

	claims := middleware.GetClaims(c)
	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, catalog.GroupID).First(&ug).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}
	if !ug.IsGroupManager() {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can update catalogs"})
		return
	}

	var payload struct {
		Name           string     `json:"name"`
		StartDate      *string    `json:"startDate"`
		EndDate        *string    `json:"endDate"`
		PercentageFees *float64   `json:"percentageFees"`
		PercentageName *string    `json:"percentageName"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&catalog).Updates(map[string]interface{}{
		"name":            payload.Name,
		"percentage_fees": payload.PercentageFees,
		"percentage_name": payload.PercentageName,
	})

	c.JSON(http.StatusOK, catalog)
}
