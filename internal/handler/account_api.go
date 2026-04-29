package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AccountAPIResponse : payload de la page Profile React, calé sur ce que
// AccountPage (templates/account.html) affiche : profil + AMAP subs +
// commandes variables récentes + alerte cotisation.
type AccountAPIResponse struct {
	User                    accountUserView   `json:"user"`
	RecentOrders            []accountOrderRow `json:"recentOrders"`
	Subscriptions           []accountSubRow   `json:"subscriptions"`
	MembershipRenewalPeriod string            `json:"membershipRenewalPeriod,omitempty"`
}

type accountUserView struct {
	ID        uint    `json:"id"`
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone,omitempty"`
	Address1  *string `json:"address1,omitempty"`
	ZipCode   *string `json:"zipCode,omitempty"`
	City      *string `json:"city,omitempty"`
}

type accountOrderRow struct {
	ProductName string  `json:"productName"`
	SmartQty    string  `json:"smartQty"`
	Total       float64 `json:"total"`
	Paid        bool    `json:"paid"`
	Date        string  `json:"date"`
}

type accountSubRow struct {
	CatalogName string `json:"catalogName"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate,omitempty"`
}

// AccountJSON sert /api/account : utilise le groupe courant (claims) pour
// les souscriptions et l'alerte de cotisation.
func (h *PagesHandler) AccountJSON(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	out := AccountAPIResponse{
		User: accountUserView{
			ID:        user.ID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Phone:     user.Phone,
			Address1:  user.Address1,
			ZipCode:   user.ZipCode,
			City:      user.City,
		},
		RecentOrders:  []accountOrderRow{},
		Subscriptions: []accountSubRow{},
	}

	// Souscriptions AMAP du groupe courant
	if claims.GroupID != 0 {
		var group model.Group
		if err := h.db.First(&group, claims.GroupID).Error; err == nil {
			var subs []model.Subscription
			h.db.Where("user_id = ?", user.ID).Preload("Catalog").Find(&subs)
			for _, s := range subs {
				if s.Catalog.GroupID != group.ID {
					continue
				}
				row := accountSubRow{
					CatalogName: s.Catalog.Name,
					StartDate:   s.StartDate.Format("02/01/2006"),
				}
				if s.EndDate != nil {
					row.EndDate = s.EndDate.Format("02/01/2006")
				}
				out.Subscriptions = append(out.Subscriptions, row)
			}

			// Alerte cotisation : groupe à HasMembership et pas d'enregistrement
			// pour l'année courante.
			if group.HasMembership {
				currentYear := time.Now().Year()
				var m model.Membership
				if err := h.db.Where("user_id = ? AND group_id = ? AND year = ?",
					user.ID, group.ID, currentYear).First(&m).Error; err != nil {
					out.MembershipRenewalPeriod = fmt.Sprintf("%d-%d", currentYear, currentYear+1)
				}
			}
		}
	}

	// Commandes variables récentes (30 derniers jours, 20 max)
	var orders []model.UserOrder
	h.db.Where("user_orders.user_id = ? AND user_orders.created_at >= ?",
		user.ID, time.Now().AddDate(0, -1, 0)).
		Preload("Product").
		Preload("Product.Catalog").
		Order("user_orders.created_at DESC").
		Limit(20).
		Find(&orders)
	for _, o := range orders {
		if o.Product.Catalog.Type != model.CatalogTypeVarOrder {
			continue
		}
		out.RecentOrders = append(out.RecentOrders, accountOrderRow{
			ProductName: o.Product.Name,
			SmartQty:    formatQty(o.Quantity, o.Product.UnitType),
			Total:       o.TotalPrice(),
			Paid:        o.Paid,
			Date:        o.CreatedAt.Format("02/01/2006"),
		})
	}

	c.JSON(http.StatusOK, out)
}
