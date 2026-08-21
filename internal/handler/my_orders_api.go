package handler

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// MyOrderView : une commande de l utilisateur, vue depuis son historique.
//
// Regroupee par distribution et non par produit : un adherent se souvient d un
// jeudi, pas d une ligne de catalogue. Le detail reste accessible en ouvrant la
// commande.
type MyOrderView struct {
	MultiDistribID uint      `json:"multiDistribId"`
	Date           time.Time `json:"date"`
	DateLabel      string    `json:"dateLabel"`
	Day            int       `json:"day"`
	Month          string    `json:"month"`
	Place          string    `json:"place"`
	NbArticles     int       `json:"nbArticles"`
	Total          float64   `json:"total"`
	// Past distingue ce qui est joue de ce qui peut encore changer : c est la
	// seule chose que l adherent cherche a savoir en ouvrant cet ecran.
	Past bool `json:"past"`
}

// MyOrdersResponse porte aussi les totaux de l annee : savoir combien on a
// soutenu ses producteurs fait partie de ce qu on vient lire.
type MyOrdersResponse struct {
	Orders    []MyOrderView `json:"orders"`
	NbOrders  int           `json:"nbOrders"`
	TotalYear float64       `json:"totalYear"`
	YearLabel int           `json:"yearLabel"`
}

// MyOrders : GET /api/my-orders — l historique des commandes du compte
// connecte, dans son groupe courant.
func (h *CompatHandler) MyOrders(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil || claims.GroupID == 0 {
		c.JSON(http.StatusOK, MyOrdersResponse{Orders: []MyOrderView{}})
		return
	}

	var commandes []model.UserOrder
	h.db.
		Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Joins("JOIN multi_distribs ON multi_distribs.id = distributions.multi_distrib_id").
		Where("user_orders.user_id = ? AND multi_distribs.group_id = ?", claims.UserID, claims.GroupID).
		Preload("Distribution.MultiDistrib.Place").
		Find(&commandes)

	frMonths := [...]string{"", "janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	frDaysFull := [...]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}

	// Regroupement par distribution : une commande, c est un jeudi, pas une
	// ligne de produit.
	parDistrib := map[uint]*MyOrderView{}
	for _, o := range commandes {
		if o.Distribution == nil || o.Distribution.MultiDistrib.ID == 0 {
			continue
		}
		md := o.Distribution.MultiDistrib
		vue, connu := parDistrib[md.ID]
		if !connu {
			debut := md.DistribStartDate
			lieu := ""
			if md.Place.Name != "" {
				lieu = md.Place.Name
			}
			vue = &MyOrderView{
				MultiDistribID: md.ID,
				Date:           debut,
				DateLabel: frDaysFull[debut.Weekday()] + " " +
					itoa(debut.Day()) + " " + frMonths[debut.Month()],
				Day:   debut.Day(),
				Month: frMonths[debut.Month()],
				Place: lieu,
				Past:  time.Now().After(debut),
			}
			parDistrib[md.ID] = vue
		}
		vue.NbArticles++
		vue.Total += o.TotalPrice()
	}

	annee := time.Now().Year()
	reponse := MyOrdersResponse{Orders: make([]MyOrderView, 0, len(parDistrib)), YearLabel: annee}
	for _, vue := range parDistrib {
		reponse.Orders = append(reponse.Orders, *vue)
		if vue.Date.Year() == annee {
			reponse.TotalYear += vue.Total
			reponse.NbOrders++
		}
	}
	// La plus recente d abord : c est celle qu on vient consulter.
	sort.Slice(reponse.Orders, func(i, j int) bool {
		return reponse.Orders[i].Date.After(reponse.Orders[j].Date)
	})

	c.JSON(http.StatusOK, reponse)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var chiffres []byte
	for n > 0 {
		chiffres = append([]byte{byte('0' + n%10)}, chiffres...)
		n /= 10
	}
	return string(chiffres)
}
