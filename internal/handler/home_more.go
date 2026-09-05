package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// maxHomeOffset borne le défilement : au-delà, on cherche des distributions
// dans un futur que personne n'a programmé. Sans cette limite, un défilement
// continu interrogerait la base indéfiniment, une période après l'autre.
const maxHomeOffset = 26

// HomeMoreFragment rend les distributions d'une période, sans la page autour.
//
// Sert au défilement continu de l'accueil : arrivé en bas, le navigateur
// demande la période suivante et l'ajoute à la liste. On répond un fragment et
// non une page, pour n'envoyer que ce qui manque.
//
// La période vide arrête le défilement : c'est le signal de fin, et il évite
// d'avoir à connaître d'avance combien de périodes existent.
func (h *PagesHandler) HomeMoreFragment(c *gin.Context) {
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "1"))
	if err != nil || offset < 1 || offset > maxHomeOffset {
		c.Status(http.StatusNoContent)
		return
	}

	pd := h.homePeriodData(c, offset)
	if pd == nil || len(pd.MultiDistribs) == 0 {
		// 204 : rien à ajouter. Le script s'arrête là.
		c.Status(http.StatusNoContent)
		return
	}

	t, err := loadTemplates("home.html", "home_more.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(c.Writer, "fragment", pd); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
