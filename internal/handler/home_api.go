package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// HomeAPIResponse est le payload JSON consommé par HomePage.tsx.
// Mapping 1-pour-1 avec les données du template home.html (à terme, le
// template HTML sera supprimé et seul cet endpoint subsiste).
type HomeAPIResponse struct {
	GroupID       uint               `json:"groupId"`
	GroupName     string             `json:"groupName"`
	GroupTxtHome  string             `json:"groupTxtHome,omitempty"`
	Offset        int                `json:"offset"`
	PeriodLabel   string             `json:"periodLabel"`
	MultiDistribs []MultiDistribView `json:"multiDistribs"`
}

// HomeJSON sert /api/home : même données que la page Go HomePage, en JSON.
// Permet à la SPA React de consommer cette page sans Go templates.
func (h *PagesHandler) HomeJSON(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	// Le groupe courant est celui des claims si présent, sinon celui du dernier
	// groupe de l'utilisateur.
	var group *model.Group
	if claims.GroupID != 0 {
		var g model.Group
		if err := h.db.First(&g, claims.GroupID).Error; err == nil {
			group = &g
		}
	}
	if group == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no current group; pick one via /user/choose"})
		return
	}

	offsetWeeks, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	frMonthsFull := [...]string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	frDaysFull := [...]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
	frDays := [...]string{"Dim", "Lun", "Mar", "Mer", "Jeu", "Ven", "Sam"}

	now := time.Now()
	weekday := int(now.Weekday())
	daysSinceSat := (weekday + 1) % 7
	periodStart := now.AddDate(0, 0, -daysSinceSat+offsetWeeks*homePeriodDays)
	periodStart = time.Date(periodStart.Year(), periodStart.Month(), periodStart.Day(), 0, 0, 0, 0, periodStart.Location())
	// Meme largeur que la page /home : sans cette constante partagee, les deux
	// interfaces annoncaient des periodes differentes au meme adherent.
	periodEnd := periodStart.AddDate(0, 0, homePeriodDays)
	periodLabel := fmt.Sprintf("Du %s %d %s %d au %s %d %s %d",
		frDays[periodStart.Weekday()], periodStart.Day(), frMonthsFull[periodStart.Month()], periodStart.Year(),
		frDays[periodEnd.Weekday()], periodEnd.Day(), frMonthsFull[periodEnd.Month()], periodEnd.Year(),
	)

	var distribs []model.MultiDistrib
	h.db.Where("group_id = ? AND distrib_start_date BETWEEN ? AND ?", group.ID, periodStart, periodEnd).
		Preload("Place").
		Preload("Distributions").
		Preload("Distributions.Catalog").
		Preload("Distributions.Catalog.Vendor").
		Order("distrib_start_date ASC").
		Find(&distribs)

	var volRoles []model.VolunteerRole
	h.db.Where("group_id = ?", group.ID).Find(&volRoles)

	views := make([]MultiDistribView, 0, len(distribs))
	for _, md := range distribs {
		start := md.DistribStartDate
		end := md.DistribEndDate

		placeAddr := ""
		if md.Place.Address != nil {
			placeAddr = *md.Place.Address
		}
		if md.Place.ZipCode != nil {
			if placeAddr != "" {
				placeAddr += " "
			}
			placeAddr += *md.Place.ZipCode
		}
		if md.Place.City != nil {
			if placeAddr != "" {
				placeAddr += " "
			}
			placeAddr += *md.Place.City
		}

		view := MultiDistribView{
			ID:           md.ID,
			Place:        md.Place.Name,
			PlaceAddress: placeAddr,
			DayOfWeek:    frDaysFull[start.Weekday()],
			Day:          fmt.Sprintf("%d", start.Day()),
			Month:        frMonthsFull[start.Month()],
			StartHour:    fmt.Sprintf("%02d:%02d", start.Hour(), start.Minute()),
			EndHour:      fmt.Sprintf("%02d:%02d", end.Hour(), end.Minute()),
			DayLabelFull: fmt.Sprintf("%s %d %s à %02d:%02d", frDaysFull[start.Weekday()], start.Day(), frMonthsFull[start.Month()], start.Hour(), start.Minute()),
			Active:       now.After(start) && now.Before(end),
			Past:         !now.Before(time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())),
		}

		// Producteurs presents. Dedupliques : deux catalogues peuvent appartenir
		// au meme producteur, qui figurerait sinon deux fois sur l ecran.
		vus := make(map[uint]bool, len(md.Distributions))
		for _, d := range md.Distributions {
			v := d.Catalog.Vendor
			if v.ID == 0 || vus[v.ID] {
				continue
			}
			vus[v.ID] = true
			ville := ""
			if v.City != nil {
				ville = *v.City
			}
			view.Vendors = append(view.Vendors, VendorView{
				ID:      v.ID,
				Name:    v.Name,
				City:    ville,
				Organic: v.Organic,
			})
		}

		// Vignettes produits (8 max sur l'ensemble des catalogues actifs)
		for _, d := range md.Distributions {
			if len(view.ProductImages) >= 8 {
				break
			}
			remaining := 8 - len(view.ProductImages)
			var prods []model.Product
			h.db.Where("catalog_id = ?", d.Catalog.ID).
				Preload("Image").Limit(remaining).Find(&prods)
			for _, p := range prods {
				url := "/img/taxo/grey/fruits-legumes.png"
				if p.Image != nil {
					url = FileURL(p.Image.ID, h.cfg.Key, p.Image.Name)
				}
				view.ProductImages = append(view.ProductImages, ProductImageView{URL: url, Name: p.Name})
			}
		}

		// État de commande, catalogue par catalogue : la clôture du jour vaut
		// par défaut, celle qu'un producteur s'est donnée fait foi pour son
		// catalogue. Il suffit qu'un catalogue reste ouvert pour proposer le
		// bouton ; le shop n'ouvrira ensuite que ceux qui le sont.
		if len(md.Distributions) > 0 {
			view.Distributions = true
			var latestEnd, nextStart *time.Time
			for _, d := range md.Distributions {
				d.MultiDistrib = md
				if d.CanOrderNow() {
					view.CanOrder = true
					if end := d.EffectiveOrderEnd(); end != nil && (latestEnd == nil || end.After(*latestEnd)) {
						latestEnd = end
					}
					continue
				}
				if start := d.EffectiveOrderStart(); start != nil && now.Before(*start) {
					if nextStart == nil || start.Before(*nextStart) {
						nextStart = start
					}
				}
			}
			switch {
			case view.CanOrder && latestEnd != nil:
				view.OrderEndDate = frDateTimeLabel(*latestEnd)
				view.OrderEndAt = latestEnd
			case !view.CanOrder && nextStart != nil:
				view.OrderNotYetOpen = true
				view.OrderStartDate = frDateTimeLabel(*nextStart)
				view.OrderStartAt = nextStart
			}
		}

		// Bénévolat : besoin = (nombre de rôles définis) - (inscrits)
		var nbRegistered int64
		h.db.Model(&model.Volunteer{}).Where("multi_distrib_id = ?", md.ID).Count(&nbRegistered)
		catalogIDs := make([]uint, 0, len(md.Distributions))
		for _, d := range md.Distributions {
			catalogIDs = append(catalogIDs, d.Catalog.ID)
		}
		rolesNeeded := make([]string, 0)
		for _, vr := range volRoles {
			if vr.CatalogID == nil {
				rolesNeeded = append(rolesNeeded, vr.Name)
			} else {
				for _, cid := range catalogIDs {
					if *vr.CatalogID == cid {
						rolesNeeded = append(rolesNeeded, vr.Name)
						break
					}
				}
			}
		}
		nbNeeded := len(rolesNeeded)
		if nbNeeded > int(nbRegistered) {
			view.VolunteerNeeded = nbNeeded - int(nbRegistered)
			view.VolunteerRoles = rolesNeeded
		}

		// Commandes du membre courant pour cette MultiDistrib
		var orders []model.UserOrder
		h.db.Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
			Joins("JOIN multi_distribs ON multi_distribs.id = distributions.multi_distrib_id").
			Where("user_orders.user_id = ? AND multi_distribs.id = ?", claims.UserID, md.ID).
			Preload("Product").
			Find(&orders)

		for _, o := range orders {
			subTotal := o.Quantity * o.ProductPrice
			total := o.TotalPrice()
			view.UserOrders = append(view.UserOrders, UserOrderView{
				ProductName: o.Product.Name,
				SmartQty:    orderQtyLabel(o.Quantity, o.Product),
				UnitPrice:   o.ProductPrice,
				SubTotal:    subTotal,
				Fees:        total - subTotal,
				Total:       total,
			})
			view.UserOrderTotal += total
		}

		views = append(views, view)
	}

	txtHome := ""
	if group.TxtHome != nil {
		txtHome = *group.TxtHome
	}

	c.JSON(http.StatusOK, HomeAPIResponse{
		GroupID:       group.ID,
		GroupName:     group.Name,
		GroupTxtHome:  txtHome,
		Offset:        offsetWeeks,
		PeriodLabel:   periodLabel,
		MultiDistribs: views,
	})
}
