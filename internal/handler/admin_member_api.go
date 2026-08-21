package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AdminMemberDetail : la fiche d un adherent, vue par qui administre le groupe.
type AdminMemberDetail struct {
	UserID  uint    `json:"userId"`
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Phone   string  `json:"phone,omitempty"`
	Address string  `json:"address,omitempty"`
	Balance float64 `json:"balance"`

	MemberSince string `json:"memberSince,omitempty"`

	MembershipYear     int     `json:"membershipYear"`
	MembershipFee      float64 `json:"membershipFee"`
	MembershipUpToDate bool    `json:"membershipUpToDate"`

	Role        string   `json:"role,omitempty"`
	Delegations []string `json:"delegations"`

	NbOrdersThisYear int     `json:"nbOrdersThisYear"`
	TotalThisYear    float64 `json:"totalThisYear"`
	NbVolunteering   int     `json:"nbVolunteering"`

	Orders []AdminMemberOrder `json:"orders"`
}

type AdminMemberOrder struct {
	MultiDistribID uint      `json:"multiDistribId"`
	Date           time.Time `json:"date"`
	DateLabel      string    `json:"dateLabel"`
	Summary        string    `json:"summary"`
	Total          float64   `json:"total"`
	Past           bool      `json:"past"`
	// Delivered n a de sens que pour une distribution passee : c est la qu il
	// repond a la question « est-il venu chercher son panier ? ».
	Delivered bool `json:"delivered"`
}

// AdminMember : GET /api/admin/members/:id.
func (h *PagesHandler) AdminMember(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil || claims.GroupID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun groupe courant"})
		return
	}
	ug := groupAccess(c, h.db, claims.UserID, claims.GroupID)
	if ug == nil || !(ug.IsGroupManager() || ug.HasRight(model.RightMembership)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "acces refuse"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifiant invalide"})
		return
	}

	// L appartenance au groupe est la condition d acces : la fiche d un
	// adherent d un autre groupe n a pas a s ouvrir ici.
	var membre model.UserGroup
	if err := h.db.Preload("User").
		Where("user_id = ? AND group_id = ?", id, claims.GroupID).
		First(&membre).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "adherent introuvable"})
		return
	}

	frMonths := [...]string{"", "janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	frDays := [...]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}
	annee := time.Now().Year()

	fiche := AdminMemberDetail{
		UserID:      membre.UserID,
		Name:        membre.User.Name(),
		Email:       membre.User.Email,
		Balance:     membre.Balance,
		Delegations: []string{},
		Orders:      []AdminMemberOrder{},
	}
	if membre.User.Phone != nil {
		fiche.Phone = *membre.User.Phone
	}
	fiche.Address = adresseLisible(membre.User)
	if !membre.User.CreatedAt.IsZero() {
		fiche.MemberSince = frMonths[membre.User.CreatedAt.Month()] + " " + itoa(membre.User.CreatedAt.Year())
	}

	switch {
	case isTechnicalManagerEmail(membre.User.Email):
		fiche.Role = model.LabelTechnicalManager
	case membre.IsGroupHead():
		fiche.Role = model.RightGroupAdmin.Label()
	default:
		for _, droit := range membre.GetRights() {
			fiche.Delegations = append(fiche.Delegations, droit.Right.Label())
		}
	}

	var adhesions []model.Membership
	h.db.Where("user_id = ? AND group_id = ?", membre.UserID, claims.GroupID).
		Order("year DESC").Find(&adhesions)
	if len(adhesions) > 0 {
		fiche.MembershipYear = adhesions[0].Year
		fiche.MembershipFee = adhesions[0].Fee
		fiche.MembershipUpToDate = adhesions[0].Year >= annee
	}

	var benevolat int64
	h.db.Model(&model.Volunteer{}).
		Joins("JOIN multi_distribs ON multi_distribs.id = volunteers.multi_distrib_id").
		Where("volunteers.user_id = ? AND multi_distribs.group_id = ?", membre.UserID, claims.GroupID).
		Count(&benevolat)
	fiche.NbVolunteering = int(benevolat)

	var commandes []model.UserOrder
	h.db.
		Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Joins("JOIN multi_distribs ON multi_distribs.id = distributions.multi_distrib_id").
		Where("user_orders.user_id = ? AND multi_distribs.group_id = ?", membre.UserID, claims.GroupID).
		Preload("Product").
		Preload("Distribution.MultiDistrib").
		Find(&commandes)

	maintenant := time.Now()
	parDistrib := map[uint]*AdminMemberOrder{}
	produits := map[uint][]string{}
	for _, o := range commandes {
		if o.Distribution == nil || o.Distribution.MultiDistrib.ID == 0 {
			continue
		}
		md := o.Distribution.MultiDistrib
		vue, connu := parDistrib[md.ID]
		if !connu {
			debut := md.DistribStartDate
			vue = &AdminMemberOrder{
				MultiDistribID: md.ID,
				Date:           debut,
				DateLabel: frDays[debut.Weekday()] + " " + itoa(debut.Day()) + " " +
					frMonths[debut.Month()],
				Past:      maintenant.After(debut),
				Delivered: true,
			}
			parDistrib[md.ID] = vue
		}
		vue.Total += o.TotalPrice()
		// Une seule ligne non remise suffit a considerer le panier en attente.
		vue.Delivered = vue.Delivered && o.HasFlag(model.OrderFlagDelivered)
		produits[md.ID] = append(produits[md.ID], o.Product.Name)
	}

	for id, vue := range parDistrib {
		vue.Summary = resumeProduits(produits[id])
		if vue.Date.Year() == annee {
			fiche.NbOrdersThisYear++
			fiche.TotalThisYear += vue.Total
		}
		fiche.Orders = append(fiche.Orders, *vue)
	}
	// La plus recente d abord.
	for i := 0; i < len(fiche.Orders); i++ {
		for j := i + 1; j < len(fiche.Orders); j++ {
			if fiche.Orders[j].Date.After(fiche.Orders[i].Date) {
				fiche.Orders[i], fiche.Orders[j] = fiche.Orders[j], fiche.Orders[i]
			}
		}
	}
	if len(fiche.Orders) > 12 {
		fiche.Orders = fiche.Orders[:12]
	}

	c.JSON(http.StatusOK, fiche)
}

// resumeProduits : « Panier de légumes, Œufs plein air + 2 autres ». Deux noms
// suffisent a reconnaitre une commande ; la liste entiere ferait un pave.
func resumeProduits(noms []string) string {
	switch {
	case len(noms) == 0:
		return ""
	case len(noms) <= 2:
		return joindre(noms, ", ")
	default:
		reste := len(noms) - 2
		return joindre(noms[:2], ", ") + " + " + itoa(reste) + " autre" + pluriel(reste)
	}
}

func joindre(elements []string, separateur string) string {
	sortie := ""
	for i, element := range elements {
		if i > 0 {
			sortie += separateur
		}
		sortie += element
	}
	return sortie
}

func pluriel(n int) string {
	if n > 1 {
		return "s"
	}
	return ""
}

func adresseLisible(u model.User) string {
	morceaux := []string{}
	if u.Address1 != nil && *u.Address1 != "" {
		morceaux = append(morceaux, *u.Address1)
	}
	ville := ""
	if u.ZipCode != nil {
		ville = *u.ZipCode
	}
	if u.City != nil && *u.City != "" {
		if ville != "" {
			ville += " "
		}
		ville += *u.City
	}
	if ville != "" {
		morceaux = append(morceaux, ville)
	}
	return joindre(morceaux, ", ")
}
