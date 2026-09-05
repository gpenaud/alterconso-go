package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// maxMembresPages borne le défilement : au-delà, c'est la recherche qu'il faut
// employer, pas la molette. Un garde-fou contre une boucle qui demanderait
// sans fin, plus qu'une limite qu'un groupe réel atteindra.
const maxMembresPages = 200

// MemberMoreFragment rend une fournée de membres pour le défilement continu.
//
// Il produit le même balisage que l'écran — le gabarit « blocsMembres » leur
// est commun — et répond 204 quand il n'y a plus personne : le script s'arrête
// alors sans avoir eu à connaître d'avance le nombre de fournées.
func (h *PagesHandler) MemberMoreFragment(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Status(http.StatusNoContent)
		return
	}
	if !pd.IsGroupManager && !pd.HasMembership {
		c.Status(http.StatusNoContent)
		return
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "2"))
	if err != nil || page < 2 || page > maxMembresPages {
		c.Status(http.StatusNoContent)
		return
	}

	h.chargerMembres(&pd, strings.TrimSpace(c.Query("q")), c.Query("filter"), page)
	if len(pd.Members) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	pd.AnneeCourante = anneeCourante()

	t, err := loadTemplates("member.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(c.Writer, "blocsMembres", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
