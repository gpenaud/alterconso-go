package handler

import (
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

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
