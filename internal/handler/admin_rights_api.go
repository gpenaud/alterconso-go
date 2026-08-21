package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
)

// AdminRightHolder : un membre et ce qu il peut faire.
type AdminRightHolder struct {
	UserID uint   `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	// Role : « Responsable de groupe », « Responsable technique », ou vide.
	// Les deux roles ne se cumulent pas avec des delegations dans l affichage :
	// ils les valent toutes.
	Role        string   `json:"role,omitempty"`
	Delegations []string `json:"delegations"`
	// Editable : le responsable technique tient son role de la configuration,
	// et ses droits ne se modifient pas depuis un ecran.
	Editable bool `json:"editable"`
}

// AdminRights : GET /api/admin/rights.
//
// Reserve a qui peut attribuer les droits — le responsable de groupe et le
// responsable technique. Voir qui detient quoi est deja une information de
// gouvernance.
func (h *PagesHandler) AdminRights(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil || claims.GroupID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun groupe courant"})
		return
	}
	ug := groupAccess(c, h.db, claims.UserID, claims.GroupID)
	if ug == nil || !ug.CanManageRights() {
		c.JSON(http.StatusForbidden, gin.H{"error": "acces refuse"})
		return
	}

	var membres []model.UserGroup
	h.db.Preload("User").
		Joins("JOIN users ON users.id = user_groups.user_id").
		Where("user_groups.group_id = ?", claims.GroupID).
		Order("users.last_name, users.first_name").
		Find(&membres)

	porteurs := make([]AdminRightHolder, 0, 4)
	for _, membre := range membres {
		technique := isTechnicalManagerEmail(membre.User.Email)
		droits := membre.GetRights()
		if len(droits) == 0 && !technique {
			continue
		}

		porteur := AdminRightHolder{
			UserID:      membre.UserID,
			Name:        membre.User.Name(),
			Email:       membre.User.Email,
			Delegations: []string{},
			Editable:    !technique,
		}

		switch {
		case technique:
			// Son role prime sur ce que porte la base : il vaut tous les
			// droits, sur tous les groupes.
			porteur.Role = model.LabelTechnicalManager
		case membre.IsGroupHead():
			porteur.Role = model.RightGroupAdmin.Label()
		default:
			for _, droit := range droits {
				porteur.Delegations = append(porteur.Delegations, droit.Right.Label())
			}
		}

		porteurs = append(porteurs, porteur)
	}

	c.JSON(http.StatusOK, gin.H{"holders": porteurs})
}
