package db

import (
	"encoding/json"
	"log"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// MigrateRightsModel convertit les droits stockés vers le modèle en vigueur :
// plus de pouvoir général, plus de rôle technique attaché à un groupe.
//
//   - « Administration » ouvrait tout le groupe d'un bloc. Il devient les deux
//     délégations qui le remplacent : distributions et paramètres. Le
//     rétrécissement est voulu — ses porteurs perdent l'accès aux membres et
//     aux catalogues, qui s'accordent maintenant droit par droit — mais il est
//     silencieux, d'où le décompte au log.
//   - « DatabaseAdmin » désignait un responsable technique par groupe. Le rôle
//     est passé en configuration, unique pour l'installation : le droit est
//     retiré, sans quoi il continuerait d'ouvrir la base à d'anciens porteurs.
//   - Le bit 0 de users.rights portait le super-administrateur global. Il est
//     éteint : le rôle ne se lit plus qu'en configuration, et un bit oublié en
//     base rendrait la révocation illusoire.
//
// Idempotente : une seconde exécution ne trouve plus rien à convertir.
func MigrateRightsModel(gdb *gorm.DB) error {
	var ugs []model.UserGroup
	if err := gdb.Find(&ugs).Error; err != nil {
		return err
	}

	converted := 0
	for _, ug := range ugs {
		next, changed := migrateRightsJSON(ug.Rights)
		if !changed {
			continue
		}
		if err := gdb.Model(&model.UserGroup{}).
			Where("user_id = ? AND group_id = ?", ug.UserID, ug.GroupID).
			Update("rights", next).Error; err != nil {
			return err
		}
		converted++
	}

	// Portée volontairement large : le bit s'éteint sur tous les comptes, y
	// compris celui du responsable technique, qui n'en a plus besoin.
	res := gdb.Exec("UPDATE users SET rights = rights & ~1 WHERE rights & 1 <> 0")
	if res.Error != nil {
		return res.Error
	}

	if converted > 0 || res.RowsAffected > 0 {
		log.Printf("[MIGRATION] droits convertis : %d appartenances, %d comptes prives du bit global",
			converted, res.RowsAffected)
	}
	return nil
}

// Anciennes valeurs, absentes du modèle courant : elles ne vivent plus que dans
// les lignes à convertir.
const (
	legacyRightAdministration model.Right = "Administration"
	legacyRightDatabaseAdmin  model.Right = "DatabaseAdmin"
)

// migrateRightsJSON transforme une liste de droits sérialisée. Retourne la
// nouvelle valeur et si elle diffère de l'ancienne.
//
// Une entrée illisible est laissée telle quelle : mieux vaut un droit qu'on
// n'interprète pas qu'un droit qu'on efface faute de le comprendre.
func migrateRightsJSON(raw string) (string, bool) {
	if raw == "" {
		return raw, false
	}
	var rights []model.UserRight
	if err := json.Unmarshal([]byte(raw), &rights); err != nil {
		return raw, false
	}

	out := make([]model.UserRight, 0, len(rights)+1)
	seen := map[model.Right]bool{}
	add := func(r model.UserRight) {
		// Les droits sans paramètre ne se répètent pas : « Administration » et
		// un « Distributions » déjà présent produiraient sinon un doublon.
		if r.Params == nil {
			if seen[r.Right] {
				return
			}
			seen[r.Right] = true
		}
		out = append(out, r)
	}

	changed := false
	for _, r := range rights {
		switch r.Right {
		case legacyRightAdministration:
			add(model.UserRight{Right: model.RightDistributions})
			add(model.UserRight{Right: model.RightParameters})
			changed = true
		case legacyRightDatabaseAdmin:
			changed = true // retiré sans remplacement
		default:
			add(r)
		}
	}
	if !changed {
		return raw, false
	}

	b, err := json.Marshal(out)
	if err != nil {
		return raw, false
	}
	return string(b), true
}
