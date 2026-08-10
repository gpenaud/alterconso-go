package db

import (
	"log"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// legacyMD5Filter sélectionne les lignes users.pass encore en MD5 legacy nu
// (32 hex minuscules) : importées du legacy ou jamais migrées. Les schémas
// bm:/b2: ne matchent pas → batch idempotent et ré-exécutable après ré-import.
const legacyMD5Filter = "pass REGEXP '^[0-9a-f]{32}$'"

// RehashLegacyPasswords enrobe en bcrypt (schéma bm:) tous les mots de passe
// encore en MD5 legacy nu, SANS mot de passe en clair. Idempotent.
func RehashLegacyPasswords(database *gorm.DB, dryRun bool, batchSize int) (processed, failed int, total int64, err error) {
	if batchSize <= 0 {
		batchSize = 200
	}

	if err = database.Model(&model.User{}).Where(legacyMD5Filter).Count(&total).Error; err != nil {
		return 0, 0, 0, err
	}
	log.Printf("rehash: %d lignes en MD5 legacy à enrober", total)
	if dryRun || total == 0 {
		return 0, 0, total, nil
	}

	for {
		var users []model.User
		if err = database.
			Select("id", "pass").
			Where(legacyMD5Filter).
			Limit(batchSize).
			Find(&users).Error; err != nil {
			return processed, failed, total, err
		}
		if len(users) == 0 {
			break
		}

		before := processed
		for _, u := range users {
			if !model.IsLegacyMD5(u.Pass) { // garde-fou
				continue
			}
			wrapped, werr := model.WrapLegacyMD5(u.Pass)
			if werr != nil {
				log.Printf("rehash: user %d wrap échoué: %v", u.ID, werr)
				failed++
				continue
			}
			if uerr := database.Model(&model.User{}).
				Where("id = ?", u.ID).
				Update("pass", wrapped).Error; uerr != nil {
				log.Printf("rehash: user %d update échoué: %v", u.ID, uerr)
				failed++
				continue
			}
			processed++
		}

		// Aucune ligne traitée alors que le filtre matche encore :
		// on s'arrête pour ne pas boucler indéfiniment sur des échecs.
		if processed == before {
			log.Printf("rehash: arrêt — %d lignes restantes non traitables", len(users))
			break
		}
		log.Printf("rehash: progression %d/%d", processed, total)
	}

	log.Printf("rehash: terminé — %d enrobés, %d en échec", processed, failed)
	return processed, failed, total, nil
}
