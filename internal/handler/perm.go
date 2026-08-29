package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

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
// couvre aussi le responsable technique via loadGroupAccess) OU s'il détient au
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
			// décision possible → on renvoie au choix de groupe, en lui
			// confiant la destination. Sans elle, un lien reçu par courrier
			// s'arrête sur l'accueil et le lecteur doit retrouver seul l'écran
			// qu'on venait de lui montrer.
			c.Redirect(http.StatusFound, "/user/choose?__redirect="+
				url.QueryEscape(c.Request.URL.RequestURI()))
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
// technique — qui passe ici avec GroupAdmin, loadGroupAccess
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
			c.Redirect(http.StatusFound, "/user/choose?__redirect="+
				url.QueryEscape(c.Request.URL.RequestURI()))
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
// groupe. Pour le responsable technique, le résultat porte toujours le droit
// GroupAdmin — même s'il existe un UserGroup en base avec des droits réduits,
// ses droits sont écrasés en mémoire pour garantir l'invariant « le responsable
// technique a perpétuellement tous les droits sur tous les groupes ».
//
// Retourne nil si l'utilisateur n'est ni membre ni responsable technique.
func loadGroupAccess(db *gorm.DB, userID, groupID uint) *model.UserGroup {
	var ug model.UserGroup
	hasReal := db.Where("user_id = ? AND group_id = ?", userID, groupID).First(&ug).Error == nil
	siteAdmin := isTechnicalManager(db, userID)

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

// technicalManagerEmail est l'adresse du responsable technique, recopiée de la
// configuration au démarrage par SetTechnicalManager.
//
// Une variable de package plutôt qu'un champ traîné de handler en handler : ce
// rôle est unique pour toute l'installation et ne change pas pendant
// l'exécution, alors qu'une vingtaine d'appels à loadGroupAccess devraient
// sinon transporter la configuration jusqu'à des handlers qui n'en ont aucun
// autre usage.
var technicalManagerEmail string

// SetTechnicalManager fixe l'adresse du responsable technique. Appelée une fois
// au montage des routes ; vide, aucun compte ne tient ce rôle.
func SetTechnicalManager(email string) {
	technicalManagerEmail = strings.ToLower(strings.TrimSpace(email))
}

// isTechnicalManagerEmail compare une adresse à celle du responsable technique.
func isTechnicalManagerEmail(email string) bool {
	if technicalManagerEmail == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(email), technicalManagerEmail)
}

// isTechnicalManager retourne true si le compte est celui du responsable
// technique. Il remplace l'ancien administrateur site-wide, dont le pouvoir
// tenait à un bit en base : une adresse en configuration ne peut pas s'octroyer
// depuis l'application.
func isTechnicalManager(db *gorm.DB, userID uint) bool {
	if technicalManagerEmail == "" {
		return false
	}
	var u model.User
	if err := db.Select("id, email").First(&u, userID).Error; err != nil {
		return false
	}
	return isTechnicalManagerEmail(u.Email)
}

// ─── Droits à titulaire unique ───────────────────────────────────────────────

// exclusiveHolder retourne le membre du groupe qui détient un droit exclusif,
// hors userID. Nil si personne ne le détient.
//
// Le responsable technique est écarté : loadGroupAccess lui accorde tous les
// droits sur tous les groupes, il figurerait sinon comme titulaire de chacun.
func exclusiveHolder(db *gorm.DB, groupID, exceptUserID uint, r model.Right) *model.UserGroup {
	var members []model.UserGroup
	if err := db.Where("group_id = ?", groupID).Preload("User").Find(&members).Error; err != nil {
		return nil
	}
	for i := range members {
		m := &members[i]
		if m.UserID == exceptUserID || isTechnicalManagerEmail(m.User.Email) {
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
// reprend. Le responsable technique reste un recours, mais plus aucun membre du
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

// safeRedirectPath filtre une destination reçue en paramètre.
//
// Seuls les chemins internes passent : « //ailleurs.example » est une URL
// absolue pour un navigateur, et la laisser passer ferait de l'écran de
// connexion un tremplin vers n'importe quel site sous le nom du nôtre.
func safeRedirectPath(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	return raw
}
