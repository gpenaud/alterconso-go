package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

type VendorHandler struct{ db *gorm.DB }

func NewVendorHandler(db *gorm.DB) *VendorHandler { return &VendorHandler{db: db} }

func (h *VendorHandler) getGroupAndCheckMembership(c *gin.Context) (*model.Group, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return nil, false
	}
	claims := middleware.GetClaims(c)
	var ug model.UserGroup
	if err := h.db.Where("user_id = ? AND group_id = ?", claims.UserID, id).First(&ug).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
		return nil, false
	}
	var group model.Group
	if err := h.db.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return nil, false
	}
	return &group, true
}

// List retourne les producteurs ayant au moins un contrat dans ce groupe.
func (h *VendorHandler) List(c *gin.Context) {
	group, ok := h.getGroupAndCheckMembership(c)
	if !ok {
		return
	}

	var vendors []model.Vendor
	h.db.
		Joins("JOIN catalogs ON catalogs.vendor_id = vendors.id").
		Where("catalogs.group_id = ?", group.ID).
		Group("vendors.id").
		Order("vendors.name").
		Find(&vendors)

	c.JSON(http.StatusOK, vendors)
}

// Create crée un nouveau producteur et l'associe au groupe via un catalogue.
func (h *VendorHandler) Create(c *gin.Context) {
	group, ok := h.getGroupAndCheckMembership(c)
	if !ok {
		return
	}

	// Vérifier que l'utilisateur est admin du groupe
	claims := middleware.GetClaims(c)
	var ug model.UserGroup
	h.db.Where("user_id = ? AND group_id = ?", claims.UserID, group.ID).First(&ug)
	if !ug.IsGroupManager() {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group admins can create vendors"})
		return
	}

	var payload struct {
		Name        string              `json:"name"        binding:"required"`
		Email       string              `json:"email"       binding:"required,email"`
		Phone       *string             `json:"phone"`
		Address1    *string             `json:"address1"`
		ZipCode     *string             `json:"zipCode"`
		City        *string             `json:"city"`
		Description *string             `json:"description"`
		LegalStatus *model.LegalStatus  `json:"legalStatus"`
		Organic     bool                `json:"organic"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vendor := model.Vendor{
		Name:        payload.Name,
		Email:       payload.Email,
		Phone:       payload.Phone,
		Address1:    payload.Address1,
		ZipCode:     payload.ZipCode,
		City:        payload.City,
		Description: payload.Description,
		LegalStatus: payload.LegalStatus,
		Organic:     payload.Organic,
	}
	if err := h.db.Create(&vendor).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, vendor)
}

// Update met à jour un producteur (réservé aux admins du groupe).
func (h *VendorHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var vendor model.Vendor
	if err := h.db.First(&vendor, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "vendor not found"})
		return
	}

	var payload struct {
		Name        string             `json:"name"`
		Email       string             `json:"email"`
		Phone       *string            `json:"phone"`
		Address1    *string            `json:"address1"`
		ZipCode     *string            `json:"zipCode"`
		City        *string            `json:"city"`
		Description *string            `json:"description"`
		LegalStatus *model.LegalStatus `json:"legalStatus"`
		Organic     bool               `json:"organic"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&vendor).Updates(map[string]interface{}{
		"name":         payload.Name,
		"email":        payload.Email,
		"phone":        payload.Phone,
		"address1":     payload.Address1,
		"zip_code":     payload.ZipCode,
		"city":         payload.City,
		"description":  payload.Description,
		"legal_status": payload.LegalStatus,
		"organic":      payload.Organic,
	})

	c.JSON(http.StatusOK, vendor)
}
