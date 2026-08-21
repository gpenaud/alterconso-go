package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AdminDistributionView : une date du calendrier, vue par qui l organise.
type AdminDistributionView struct {
	ID              uint       `json:"id"`
	StartAt         time.Time  `json:"startAt"`
	DayOfWeek       string     `json:"dayOfWeek"`
	Day             int        `json:"day"`
	Month           string     `json:"month"`
	StartHour       string     `json:"startHour"`
	EndHour         string     `json:"endHour"`
	Place           string     `json:"place"`
	Past            bool       `json:"past"`
	Open            bool       `json:"open"`
	OrderEndAt      *time.Time `json:"orderEndAt,omitempty"`
	OrderStartLabel string     `json:"orderStartLabel,omitempty"`
	NbVendors       int        `json:"nbVendors"`
	NbOrders        int        `json:"nbOrders"`
	Total           float64    `json:"total"`
	VolunteerNeeded int        `json:"volunteerNeeded"`
}

// AdminDistributions : GET /api/admin/distributions.
//
// Les six prochaines et les six dernieres. Assez pour tenir un trimestre sous
// les yeux sans charger deux ans d historique — le calendrier sert a preparer,
// pas a archiver.
func (h *PagesHandler) AdminDistributions(c *gin.Context) {
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

	maintenant := time.Now()
	var mds []model.MultiDistrib
	h.db.Where("group_id = ?", claims.GroupID).
		Preload("Place").
		Preload("Distributions.Catalog").
		Order("distrib_start_date DESC").
		Limit(12).
		Find(&mds)

	frMonths := [...]string{"", "janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	frDays := [...]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}

	var roles []model.VolunteerRole
	h.db.Where("group_id = ?", claims.GroupID).Find(&roles)

	vues := make([]AdminDistributionView, 0, len(mds))
	for _, md := range mds {
		debut := md.DistribStartDate
		vue := AdminDistributionView{
			ID:        md.ID,
			StartAt:   debut,
			DayOfWeek: frDays[debut.Weekday()],
			Day:       debut.Day(),
			Month:     frMonths[debut.Month()],
			StartHour: debut.Format("15h04"),
			EndHour:   md.DistribEndDate.Format("15h04"),
			Place:     md.Place.Name,
			Past:      maintenant.After(md.DistribEndDate),
		}

		vendeurs := map[uint]bool{}
		for _, d := range md.Distributions {
			if d.Catalog.VendorID != 0 {
				vendeurs[d.Catalog.VendorID] = true
			}
			// Une distribution est ouverte des qu un de ses catalogues accepte
			// encore des commandes : les dates se reglent catalogue par
			// catalogue.
			if d.OrderEndDate != nil && maintenant.Before(*d.OrderEndDate) &&
				(d.OrderStartDate == nil || maintenant.After(*d.OrderStartDate)) {
				vue.Open = true
				if vue.OrderEndAt == nil || d.OrderEndDate.After(*vue.OrderEndAt) {
					vue.OrderEndAt = d.OrderEndDate
				}
			}
			if d.OrderStartDate != nil && maintenant.Before(*d.OrderStartDate) && vue.OrderStartLabel == "" {
				vue.OrderStartLabel = frDateTimeLabel(*d.OrderStartDate)
			}
		}
		vue.NbVendors = len(vendeurs)

		var commandes []model.UserOrder
		h.db.
			Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
			Where("distributions.multi_distrib_id = ?", md.ID).
			Find(&commandes)
		adherents := map[uint]bool{}
		for _, o := range commandes {
			adherents[o.UserID] = true
			vue.Total += o.TotalPrice()
		}
		vue.NbOrders = len(adherents)

		if !vue.Past {
			var inscrits int64
			h.db.Model(&model.Volunteer{}).Where("multi_distrib_id = ?", md.ID).Count(&inscrits)
			if manque := len(roles) - int(inscrits); manque > 0 {
				vue.VolunteerNeeded = manque
			}
		}

		vues = append(vues, vue)
	}

	c.JSON(http.StatusOK, gin.H{"distributions": vues})
}
