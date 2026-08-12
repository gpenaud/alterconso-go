package handler

import (
	"encoding/json"
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
	// Une route qui réclame le droit technique ne s'ouvre qu'à qui le détient
	// vraiment : les « droits administrateur » ouvrent tout le reste, mais
	// s'arrêtent là. Le test précède IsGroupManager, qui les couvre.
	for _, r := range rights {
		if r == model.RightDatabaseAdmin {
			return ug.CanAdminDatabase()
		}
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

// RequireRightsManagement garde les pages d'attribution des droits.
//
// Plus étroit que RequireGroupRight() : responsable de groupe, responsable
// technique, et le superadmin — qui passe ici avec GroupAdmin, loadGroupAccess
// le lui accordant sur tous les groupes. Les « droits administrateur » en sont
// exclus : pouvoir se désigner responsable technique leur ouvrirait la base de
// données, que ce droit leur refuse.
func (h *PagesHandler) RequireRightsManagement() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			redirect := url.QueryEscape(c.Request.URL.RequestURI())
			c.Redirect(http.StatusFound, "/user/login?__redirect="+redirect)
			c.Abort()
			return
		}
		if claims.GroupID == 0 {
			c.Redirect(http.StatusFound, "/user/choose")
			c.Abort()
			return
		}
		ug := groupAccess(c, h.db, claims.UserID, claims.GroupID)
		if ug == nil || !ug.CanManageRights() {
			c.String(http.StatusForbidden, "accès refusé : seuls le responsable de groupe et le responsable technique attribuent les droits")
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

// ─── Droits à titulaire unique ───────────────────────────────────────────────

// exclusiveHolder retourne le membre du groupe qui détient un droit exclusif,
// hors userID. Nil si personne ne le détient.
//
// Le superadmin global est écarté : loadGroupAccess lui accorde tous les droits
// sur tous les groupes, il figurerait sinon comme titulaire de chacun.
func exclusiveHolder(db *gorm.DB, groupID, exceptUserID uint, r model.Right) *model.UserGroup {
	var members []model.UserGroup
	if err := db.Where("group_id = ?", groupID).Preload("User").Find(&members).Error; err != nil {
		return nil
	}
	for i := range members {
		m := &members[i]
		if m.UserID == exceptUserID || m.User.IsAdmin() {
			continue
		}
		if m.HasRight(r) {
			return m
		}
	}
	return nil
}

// transferExclusiveRights retire aux autres membres du groupe les droits
// exclusifs que userID vient de recevoir : un tel droit n'a qu'un titulaire, le
// lui accorder le prend donc à son prédécesseur.
//
// Retourne, par droit transféré, le nom de la personne dépossédée — l'appelant
// le rapporte, faute de quoi un responsable perdrait son rôle sans que rien ne
// le dise.
func transferExclusiveRights(db *gorm.DB, groupID, userID uint, granted []model.UserRight) (map[model.Right]string, error) {
	transfers := make(map[model.Right]string)

	for _, r := range granted {
		if !model.IsExclusiveRight(r.Right) {
			continue
		}
		holder := exclusiveHolder(db, groupID, userID, r.Right)
		if holder == nil {
			continue
		}

		kept := make([]model.UserRight, 0, len(holder.GetRights()))
		for _, hr := range holder.GetRights() {
			if hr.Right != r.Right {
				kept = append(kept, hr)
			}
		}
		encoded, err := json.Marshal(kept)
		if err != nil {
			return transfers, err
		}
		if err := db.Model(&model.UserGroup{}).
			Where("user_id = ? AND group_id = ?", holder.UserID, groupID).
			Update("rights", string(encoded)).Error; err != nil {
			return transfers, err
		}
		transfers[r.Right] = holder.User.Name()
	}
	return transfers, nil
}

// leavesGroupWithoutManager indique que l'enregistrement priverait le groupe de
// tout responsable : le titulaire actuel se retire le rôle et personne ne le
// reprend. Le superadmin global reste un recours, mais plus aucun membre du
// groupe ne pourrait en administrer les droits.
func leavesGroupWithoutManager(db *gorm.DB, groupID, userID uint, granted []model.UserRight) bool {
	for _, r := range granted {
		if r.Right == model.RightGroupAdmin {
			return false // il le garde ou le reçoit
		}
	}
	// Il ne l'a pas dans ce qu'on enregistre : reste-t-il quelqu'un d'autre ?
	return exclusiveHolder(db, groupID, userID, model.RightGroupAdmin) == nil &&
		hasRightInGroup(db, groupID, userID, model.RightGroupAdmin)
}

// hasRightInGroup dit si le membre détient actuellement le droit en base.
func hasRightInGroup(db *gorm.DB, groupID, userID uint, r model.Right) bool {
	var ug model.UserGroup
	if err := db.Where("user_id = ? AND group_id = ?", userID, groupID).First(&ug).Error; err != nil {
		return false
	}
	return ug.HasRight(r)
}
