package db

import (
	"encoding/json"
	"log"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// restoreDelegatedRightsName nomme la correction dans applied_migrations. La
// date y figure : c'est un evenement ponctuel, rattache a la conversion des
// droits du 18 aout 2026, et non une regle permanente.
const restoreDelegatedRightsName = "2026-08-18-rend-membres-et-catalogues"

// RestoreDelegatedMembersAndCatalogs rend « gestion des membres » et « gestion
// des catalogues » aux comptes que la conversion des droits a retrecis.
//
// Les anciens « droits administrateur » et « DatabaseAdmin » ouvraient le
// groupe entier. Leur conversion n'a laisse que les deux delegations qui les
// remplacent, retirant du meme coup les membres et les catalogues a des gens
// qui s'en servaient. La correction ne rend que ces deux acces : le reste du
// decoupage est voulu.
//
// Elle ne vise qu'une signature : les deux delegations ensemble, laissees par
// un « Administration » converti.
//
// La liste vide en faisait partie a l'origine, tenue pour la trace d'un
// « DatabaseAdmin » retire. C'etait faux — « [] » est l'etat ordinaire d'un
// adherent sans aucun droit — et 88 appartenances ont ete elargies au lieu de 4
// avant que RevertOverreachingMembersAndCatalogs ne repare. Les anciens
// porteurs de « DatabaseAdmin » seul restent donc a rattraper a la main : leur
// etat d'avant conversion ne se lit plus nulle part.
//
// Ponctuelle : passee une fois, elle ne se rejouera pas, sans quoi elle
// rendrait aussi ces droits a qui on vient de les retirer.
func RestoreDelegatedMembersAndCatalogs(gdb *gorm.DB) error {
	return runOnce(gdb, restoreDelegatedRightsName, func(gdb *gorm.DB) error {
		var ugs []model.UserGroup
		if err := gdb.Preload("User").Find(&ugs).Error; err != nil {
			return err
		}

		restored := 0
		for _, ug := range ugs {
			next, changed := restoreMembersAndCatalogs(ug.Rights)
			if !changed {
				continue
			}
			if err := gdb.Model(&model.UserGroup{}).
				Where("user_id = ? AND group_id = ?", ug.UserID, ug.GroupID).
				Update("rights", next).Error; err != nil {
				return err
			}
			restored++
			// Nommer les comptes touches : la conversion precedente n'avait
			// laisse qu'un decompte, impossible a recouper apres coup.
			log.Printf("[MIGRATION] membres et catalogues rendus a %s (groupe %d)",
				ug.User.Email, ug.GroupID)
		}
		log.Printf("[MIGRATION] %d appartenances corrigees", restored)
		return nil
	})
}

// restoreMembersAndCatalogs ajoute les deux droits a une liste qui porte la
// signature d'une conversion. Retourne la nouvelle valeur et si elle a change.
func restoreMembersAndCatalogs(raw string) (string, bool) {
	if raw == "" {
		return raw, false
	}
	var rights []model.UserRight
	if err := json.Unmarshal([]byte(raw), &rights); err != nil {
		return raw, false
	}

	has := func(r model.Right) bool {
		for _, x := range rights {
			if x.Right == r {
				return true
			}
		}
		return false
	}

	converti := has(model.RightDistributions) && has(model.RightParameters)
	if !converti {
		return raw, false
	}

	changed := false
	// Un droit de catalogue restreint a certains catalogues est laisse tel
	// quel : l'elargir a tous depasserait la reparation.
	if !has(model.RightMembership) {
		rights = append(rights, model.UserRight{Right: model.RightMembership})
		changed = true
	}
	if !has(model.RightCatalogAdmin) {
		rights = append(rights, model.UserRight{Right: model.RightCatalogAdmin})
		changed = true
	}
	if !changed {
		return raw, false
	}

	b, err := json.Marshal(rights)
	if err != nil {
		return raw, false
	}
	return string(b), true
}
