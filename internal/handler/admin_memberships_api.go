package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AdminMembershipView : l etat d adhesion d un adherent.
type AdminMembershipView struct {
	UserID uint   `json:"userId"`
	Name   string `json:"name"`
	// LastYear vaut 0 quand la personne n a jamais adhere : c est autre chose
	// qu un retard, et l ecran doit pouvoir les distinguer.
	LastYear int  `json:"lastYear"`
	UpToDate bool `json:"upToDate"`
	// NbOrdersThisYear separe celui qui a oublie de payer mais commande chaque
	// semaine, de celui qui a quitte le groupe sans le dire.
	NbOrdersThisYear int `json:"nbOrdersThisYear"`
}

// AdminMemberships : GET /api/admin/memberships.
func (h *PagesHandler) AdminMemberships(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil || claims.GroupID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun groupe courant"})
		return
	}
	ug := groupAccess(c, h.db, claims.UserID, claims.GroupID)
	if ug == nil || !(ug.IsGroupManager() || ug.CanManageParameters() || ug.HasRight(model.RightMembership)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "acces refuse"})
		return
	}

	var groupe model.Group
	if err := h.db.First(&groupe, claims.GroupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "groupe introuvable"})
		return
	}

	annee := time.Now().Year()

	var membres []model.UserGroup
	h.db.Preload("User").
		Joins("JOIN users ON users.id = user_groups.user_id").
		Where("user_groups.group_id = ?", claims.GroupID).
		Order("users.last_name, users.first_name").
		Find(&membres)

	var adhesions []model.Membership
	h.db.Where("group_id = ?", claims.GroupID).Find(&adhesions)
	derniereAnnee := map[uint]int{}
	var collecte float64
	for _, a := range adhesions {
		if a.Year > derniereAnnee[a.UserID] {
			derniereAnnee[a.UserID] = a.Year
		}
		if a.Year == annee {
			collecte += a.Fee
		}
	}

	// Une commande passee cette annee, quelle que soit la distribution.
	var commandes []model.UserOrder
	h.db.
		Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Joins("JOIN multi_distribs ON multi_distribs.id = distributions.multi_distrib_id").
		Where("multi_distribs.group_id = ? AND YEAR(multi_distribs.distrib_start_date) = ?", claims.GroupID, annee).
		Find(&commandes)
	commandesParUtilisateur := map[uint]map[uint]bool{}
	for _, o := range commandes {
		if commandesParUtilisateur[o.UserID] == nil {
			commandesParUtilisateur[o.UserID] = map[uint]bool{}
		}
		if o.DistributionID != nil {
			commandesParUtilisateur[o.UserID][*o.DistributionID] = true
		}
	}

	vues := make([]AdminMembershipView, 0, len(membres))
	aJour, enRetard := 0, 0
	for _, membre := range membres {
		derniere := derniereAnnee[membre.UserID]
		vue := AdminMembershipView{
			UserID:           membre.UserID,
			Name:             membre.User.Name(),
			LastYear:         derniere,
			UpToDate:         derniere >= annee,
			NbOrdersThisYear: len(commandesParUtilisateur[membre.UserID]),
		}
		if vue.UpToDate {
			aJour++
		} else {
			enRetard++
		}
		vues = append(vues, vue)
	}

	frais := 0.0
	if groupe.MembershipFee != nil {
		frais = float64(*groupe.MembershipFee)
	}
	renouvellement := ""
	if groupe.MembershipRenewalDate != nil {
		renouvellement = frJourEtMois(*groupe.MembershipRenewalDate)
	}

	c.JSON(http.StatusOK, gin.H{
		"members":       vues,
		"fee":           frais,
		"renewalDate":   renouvellement,
		"collectedYear": collecte,
		"upToDate":      aJour,
		"late":          enRetard,
		"year":          annee,
		"hasMembership": groupe.HasMembership,
	})
}

// frJourEtMois : « 31 décembre », sans l année ni le jour de la semaine — une
// date de renouvellement se répète chaque année, et l afficher datée
// laisserait croire à une échéance unique.
func frJourEtMois(t time.Time) string {
	frMonths := [...]string{"", "janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	return itoa(t.Day()) + " " + frMonths[t.Month()]
}
