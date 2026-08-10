package handler

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// ctxGroupAccessKey : l'UserGroup résolu (droits du user dans le groupe
// courant) est mis en cache dans le contexte Gin par le middleware
// d'autorisation, puis réutilisé par buildPageData pour éviter une seconde
// requête loadGroupAccess par requête HTTP.
const ctxGroupAccessKey = "groupAccess"

// groupAccess retourne l'UserGroup pertinent en privilégiant le cache de
// contexte (posé par RequireGroupRight) ; sinon le charge et le met en cache.
func groupAccess(c *gin.Context, db *gorm.DB, userID, groupID uint) *model.UserGroup {
	if v, ok := c.Get(ctxGroupAccessKey); ok {
		if ug, ok := v.(*model.UserGroup); ok {
			return ug
		}
	}
	ug := loadGroupAccess(db, userID, groupID)
	if ug != nil {
		c.Set(ctxGroupAccessKey, ug)
	}
	return ug
}

// authorize est la décision d'autorisation PURE (testable sans DB ni HTTP) :
// accès accordé si l'utilisateur est gestionnaire du groupe (GroupAdmin, qui
// couvre aussi le superadmin site-wide via loadGroupAccess) OU s'il détient au
// moins un des droits requis. Une liste de droits vide ⇒ gestionnaire requis.
// ug nil (ni membre ni admin du groupe) ⇒ toujours refusé (fail-closed).
func authorize(ug *model.UserGroup, rights []model.Right) bool {
	if ug == nil {
		return false
	}
	if ug.IsGroupManager() {
		return true
	}
	for _, r := range rights {
		if ug.HasRight(r) {
			return true
		}
	}
	return false
}

// RequireGroupRight est le middleware d'autorisation central pour les pages
// HTML. Il s'insère APRÈS middleware.PageAuth (qui a posé les claims) et AVANT
// le handler. Réservé aux routes admin ; les routes membre n'en portent pas.
//
//   - aucun droit passé        ⇒ gestionnaire (GroupAdmin) requis
//   - un ou plusieurs droits   ⇒ gestionnaire OU l'un de ces droits
//
// Fail-closed : claims absents, aucun groupe courant, ou non-membre ⇒ refus.
func (h *PagesHandler) RequireGroupRight(rights ...model.Right) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			// PageAuth aurait dû filtrer ; défense en profondeur.
			redirect := url.QueryEscape(c.Request.URL.RequestURI())
			c.Redirect(http.StatusFound, "/user/login?__redirect="+redirect)
			c.Abort()
			return
		}
		if claims.GroupID == 0 {
			// Les droits sont par groupe : sans groupe courant, pas de
			// décision possible → on renvoie au choix de groupe.
			c.Redirect(http.StatusFound, "/user/choose")
			c.Abort()
			return
		}

		ug := groupAccess(c, h.db, claims.UserID, claims.GroupID)
		if !authorize(ug, rights) {
			c.String(http.StatusForbidden, "accès refusé")
			c.Abort()
			return
		}
		c.Next()
	}
}

// fullRightsJSON est le JSON utilisé pour donner tous les droits via le seul
// droit GroupAdmin (qui implique l'accès à toutes les sous-fonctions).
const fullRightsJSON = `[{"right":"GroupAdmin"}]`

// loadGroupAccess retourne l'UserGroup pertinent pour une demande d'accès au
// groupe. Pour un admin site-wide, le résultat porte toujours le droit
// GroupAdmin — même s'il existe un UserGroup en base avec des droits réduits,
// ses droits sont écrasés en mémoire pour garantir l'invariant « le superadmin
// a perpétuellement tous les droits sur tous les groupes ».
//
// Retourne nil si l'utilisateur n'est ni membre ni admin site-wide.
func loadGroupAccess(db *gorm.DB, userID, groupID uint) *model.UserGroup {
	var ug model.UserGroup
	hasReal := db.Where("user_id = ? AND group_id = ?", userID, groupID).First(&ug).Error == nil
	siteAdmin := isSiteAdmin(db, userID)

	if !hasReal {
		if !siteAdmin {
			return nil
		}
		return &model.UserGroup{
			UserID:  userID,
			GroupID: groupID,
			Rights:  fullRightsJSON,
		}
	}

	if siteAdmin {
		// Conserve la balance et les autres champs, mais force les droits.
		ug.Rights = fullRightsJSON
	}
	return &ug
}

// isSiteAdmin retourne true si l'utilisateur est administrateur site-wide.
func isSiteAdmin(db *gorm.DB, userID uint) bool {
	var u model.User
	if err := db.Select("id, rights").First(&u, userID).Error; err != nil {
		return false
	}
	return u.IsAdmin()
}
