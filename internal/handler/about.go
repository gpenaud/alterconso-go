package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AboutData struct {
	PageData
	// TechnicalManagerEmail : à qui écrire quand quelque chose cloche dans
	// l'application elle-même, par opposition au groupe. Vide, la mention ne
	// s'affiche pas plutôt que d'offrir un lien vers nulle part.
	TechnicalManagerEmail string
	AppVersion            string
}

// AboutPage : les mentions qui vivaient dans le pied de page.
//
// Déplacées ici parce qu'un pied de page pleine largeur suivait le contenu de
// chaque écran, poussant vers le bas ce qui devait s'y trouver — le
// déclencheur du défilement continu de l'accueil, notamment. Ces informations
// se consultent une fois ; elles n'avaient pas à occuper le bas de toutes les
// pages.
func (h *PagesHandler) AboutPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil {
		c.Redirect(http.StatusFound, "/user/login?__redirect=/apropos")
		return
	}

	data := AboutData{PageData: pd}
	data.Title = "À propos"
	data.Category = "account"
	data.Breadcrumb = []BreadcrumbItem{{Name: "À propos", Link: "/apropos"}}
	data.TechnicalManagerEmail = h.cfg.TechnicalManager.Email

	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "about.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
