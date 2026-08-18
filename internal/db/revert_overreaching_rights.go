package db

import (
	"encoding/json"
	"log"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

const revertOverreachingRightsName = "2026-08-18-annule-l-elargissement-errone"

// RevertOverreachingMembersAndCatalogs annule un elargissement de droits errone.
//
// La correction precedente visait les appartenances retrecies par la conversion
// du modele de droits. Elle tenait « rights = [] » pour la signature d'un
// « DatabaseAdmin » retire, alors que c'est l'etat ordinaire d'un adherent sans
// aucun droit : 88 appartenances ont ete elargies au lieu des 4 attendues, et
// des adherents se sont retrouves gestionnaires des membres et des catalogues.
//
// La reparation vise la signature exacte que cet elargissement a laissee —
// « Membership » puis « CatalogAdmin », sans parametres et sans rien d'autre —
// et rend la liste vide. Les quatre appartenances legitimement corrigees
// portent aussi les deux delegations : elles ne repondent pas a ce motif et
// gardent leurs droits.
//
// Un compte qui portait deja exactement ces deux droits avant l'incident serait
// remis a zero a tort. Les comptes touches sont donc nommes au log, a recouper
// avec ceux de l'elargissement.
func RevertOverreachingMembersAndCatalogs(gdb *gorm.DB) error {
	return runOnce(gdb, revertOverreachingRightsName, func(gdb *gorm.DB) error {
		var ugs []model.UserGroup
		if err := gdb.Preload("User").Find(&ugs).Error; err != nil {
			return err
		}

		reverted := 0
		for _, ug := range ugs {
			if !isOverreachingGrant(ug.Rights) {
				continue
			}
			if err := gdb.Model(&model.UserGroup{}).
				Where("user_id = ? AND group_id = ?", ug.UserID, ug.GroupID).
				Update("rights", "[]").Error; err != nil {
				return err
			}
			reverted++
			log.Printf("[MIGRATION] droits errones retires a %s (groupe %d)",
				ug.User.Email, ug.GroupID)
		}
		log.Printf("[MIGRATION] %d appartenances remises a zero", reverted)
		return nil
	})
}

// isOverreachingGrant reconnait la signature exacte laissee par
// l'elargissement : les deux droits, dans cet ordre, sans parametres, et rien
// d'autre. Toute liste plus riche a une autre origine et n'est pas touchee.
func isOverreachingGrant(raw string) bool {
	if raw == "" {
		return false
	}
	var rights []model.UserRight
	if err := json.Unmarshal([]byte(raw), &rights); err != nil {
		return false
	}
	if len(rights) != 2 {
		return false
	}
	return rights[0].Right == model.RightMembership && rights[0].Params == nil &&
		rights[1].Right == model.RightCatalogAdmin && rights[1].Params == nil
}
