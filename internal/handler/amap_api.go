package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AmapAPIResponse : payload de la page Producteurs React, calé sur ce que
// AmapPage (templates/amap.html) affiche : liste de vendors avec leurs
// catalogues + coordinateur, contact principal du groupe, droit admin.
type AmapAPIResponse struct {
	Group          amapGroupView   `json:"group"`
	Contact        *amapPersonView `json:"contact,omitempty"`
	Vendors        []amapVendorRow `json:"vendors"`
	IsGroupManager bool            `json:"isGroupManager"`
}

type amapGroupView struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type amapPersonView struct {
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	Email     string  `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

type amapCatalogRow struct {
	ID            uint               `json:"id"`
	Name          string             `json:"name"`
	ProductImages []ProductImageView `json:"productImages"`
	Coordinator   *amapPersonView    `json:"coordinator,omitempty"`
}

type amapVendorRow struct {
	ID       uint             `json:"id"`
	Name     string           `json:"name"`
	City     string           `json:"city,omitempty"`
	ZipCode  string           `json:"zipCode,omitempty"`
	Catalogs []amapCatalogRow `json:"catalogs"`
}

// AmapJSON sert /api/amap : utilise le groupe courant des claims.
func (h *PagesHandler) AmapJSON(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil || claims.GroupID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no current group"})
		return
	}
	var group model.Group
	if err := h.db.First(&group, claims.GroupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	ug := loadGroupAccess(h.db, claims.UserID, group.ID)

	// Catalogues + vendor + coordinateur, ordre stable par 1re apparition.
	var catalogs []model.Catalog
	h.db.Where("group_id = ?", group.ID).
		Preload("Vendor").
		Preload("Contact").
		Find(&catalogs)

	vendorOrder := []uint{}
	vendorMap := make(map[uint]*amapVendorRow)
	for _, cat := range catalogs {
		v := cat.Vendor
		if _, ok := vendorMap[v.ID]; !ok {
			row := &amapVendorRow{ID: v.ID, Name: v.Name}
			if v.City != nil {
				row.City = *v.City
			}
			if v.ZipCode != nil {
				row.ZipCode = *v.ZipCode
			}
			vendorMap[v.ID] = row
			vendorOrder = append(vendorOrder, v.ID)
		}
		var prods []model.Product
		h.db.Where("catalog_id = ?", cat.ID).Preload("Image").Limit(5).Find(&prods)
		imgs := make([]ProductImageView, 0, len(prods))
		for _, p := range prods {
			url := "/img/taxo/grey/fruits-legumes.png"
			if p.Image != nil {
				url = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
			}
			imgs = append(imgs, ProductImageView{URL: url, Name: p.Name})
		}
		var coord *amapPersonView
		if cat.Contact != nil {
			coord = &amapPersonView{
				FirstName: cat.Contact.FirstName,
				LastName:  cat.Contact.LastName,
				Email:     cat.Contact.Email,
				Phone:     cat.Contact.Phone,
			}
		}
		vendorMap[v.ID].Catalogs = append(vendorMap[v.ID].Catalogs, amapCatalogRow{
			ID:            cat.ID,
			Name:          cat.Name,
			ProductImages: imgs,
			Coordinator:   coord,
		})
	}
	vendors := make([]amapVendorRow, 0, len(vendorOrder))
	for _, id := range vendorOrder {
		vendors = append(vendors, *vendorMap[id])
	}

	out := AmapAPIResponse{
		Group:          amapGroupView{ID: group.ID, Name: group.Name},
		Vendors:        vendors,
		IsGroupManager: ug != nil && ug.IsGroupManager(),
	}
	if group.ContactID != nil {
		var contact model.User
		if err := h.db.First(&contact, *group.ContactID).Error; err == nil {
			out.Contact = &amapPersonView{
				FirstName: contact.FirstName,
				LastName:  contact.LastName,
				Email:     contact.Email,
				Phone:     contact.Phone,
			}
		}
	}

	c.JSON(http.StatusOK, out)
}
