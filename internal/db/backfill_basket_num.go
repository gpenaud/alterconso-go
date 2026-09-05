package db

import (
	"log"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// BackfillBasketNumbers rattrape ce que la réécriture avait laissé de côté : la
// numérotation des paniers.
//
// L'application d'origine donnait à chaque panier — un adhérent, une
// distribution — un numéro figé, que tous les producteurs présents lisaient à
// l'identique. La réécriture a conservé le panier mais perdu son numéro, et les
// écrans se sont rabattus sur un rang recalculé à l'affichage : filtré par
// producteur, il redonnait à la même personne un numéro différent chez chacun.
//
// Cette migration remet la base en état d'y répondre :
//
//   - les paniers en double pour un même couple (adhérent, distribution) sont
//     fusionnés, sans quoi l'adhérent porterait deux numéros ;
//   - les commandes sans panier en reçoivent un — c'était le cas de toutes
//     celles saisies par un responsable, le rattachement n'existant que sur le
//     chemin de la boutique ;
//   - les paniers non numérotés le sont, par distribution, dans l'ordre où ils
//     ont été créés : c'est l'ordre qu'appliquait l'application d'origine ;
//   - les deux unicités sont posées en index, pour que la base refuse
//     désormais ce que cette fonction vient de réparer.
//
// Idempotente : une seconde exécution ne trouve plus rien à reprendre.
func BackfillBasketNumbers(gdb *gorm.DB) error {
	if err := mergeDuplicateBaskets(gdb); err != nil {
		return err
	}
	if err := createMissingBaskets(gdb); err != nil {
		return err
	}
	if err := attachOrdersToBaskets(gdb); err != nil {
		return err
	}
	if err := numberBaskets(gdb); err != nil {
		return err
	}
	return enforceBasketUniqueness(gdb)
}

// mergeDuplicateBaskets ramène chaque couple (adhérent, distribution) à un
// panier unique — le plus ancien — et lui reporte commandes et opérations.
func mergeDuplicateBaskets(gdb *gorm.DB) error {
	type doublon struct {
		UserID         uint
		MultiDistribID uint
		KeepID         uint
	}
	var doublons []doublon
	if err := gdb.Raw(`
		SELECT user_id, multi_distrib_id, MIN(id) AS keep_id
		FROM baskets
		GROUP BY user_id, multi_distrib_id
		HAVING COUNT(*) > 1`).Scan(&doublons).Error; err != nil {
		return err
	}

	for _, d := range doublons {
		var perdants []uint
		if err := gdb.Raw(`
			SELECT id FROM baskets
			WHERE user_id = ? AND multi_distrib_id = ? AND id <> ?`,
			d.UserID, d.MultiDistribID, d.KeepID).Scan(&perdants).Error; err != nil {
			return err
		}
		if len(perdants) == 0 {
			continue
		}
		if err := gdb.Exec(`UPDATE user_orders SET basket_id = ? WHERE basket_id IN ?`,
			d.KeepID, perdants).Error; err != nil {
			return err
		}
		if err := gdb.Exec(`UPDATE operations SET basket_id = ? WHERE basket_id IN ?`,
			d.KeepID, perdants).Error; err != nil {
			return err
		}
		if err := gdb.Exec(`DELETE FROM baskets WHERE id IN ?`, perdants).Error; err != nil {
			return err
		}
	}
	if len(doublons) > 0 {
		log.Printf("BackfillBasketNumbers: %d paniers en double fusionnés", len(doublons))
	}
	return nil
}

// createMissingBaskets crée le panier des adhérents qui ont commandé sans en
// avoir un. Sa date de création est celle de leur première commande, pour que
// la numérotation qui suit respecte l'ordre dans lequel ils ont commandé.
func createMissingBaskets(gdb *gorm.DB) error {
	res := gdb.Exec(`
		INSERT INTO baskets (created_at, user_id, multi_distrib_id, num)
		SELECT MIN(uo.created_at), uo.user_id, d.multi_distrib_id, 0
		FROM user_orders uo
		JOIN distributions d ON d.id = uo.distribution_id
		LEFT JOIN baskets b
			ON b.user_id = uo.user_id AND b.multi_distrib_id = d.multi_distrib_id
		WHERE b.id IS NULL
		GROUP BY uo.user_id, d.multi_distrib_id`)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("BackfillBasketNumbers: %d paniers manquants créés", res.RowsAffected)
	}
	return nil
}

// attachOrdersToBaskets rattache les commandes laissées sans panier, ainsi que
// celles dont le panier a disparu — la migration depuis l'ancienne base a
// repris les identifiants de panier sans reprendre les paniers eux-mêmes.
func attachOrdersToBaskets(gdb *gorm.DB) error {
	res := gdb.Exec(`
		UPDATE user_orders uo
		JOIN distributions d ON d.id = uo.distribution_id
		JOIN baskets b
			ON b.user_id = uo.user_id AND b.multi_distrib_id = d.multi_distrib_id
		SET uo.basket_id = b.id
		WHERE uo.basket_id IS NULL
		   OR NOT EXISTS (SELECT 1 FROM baskets b2 WHERE b2.id = uo.basket_id)`)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("BackfillBasketNumbers: %d commandes rattachées à leur panier", res.RowsAffected)
	}
	return nil
}

// numberBaskets numérote les paniers restés à zéro, distribution par
// distribution, en repartant du plus grand numéro déjà attribué : les numéros
// déjà connus des adhérents ne bougent pas.
func numberBaskets(gdb *gorm.DB) error {
	var distribs []uint
	if err := gdb.Model(&model.Basket{}).
		Where("num = 0").
		Distinct().
		Pluck("multi_distrib_id", &distribs).Error; err != nil {
		return err
	}

	total := 0
	for _, mdID := range distribs {
		var max int
		if err := gdb.Model(&model.Basket{}).
			Where("multi_distrib_id = ?", mdID).
			Select("COALESCE(MAX(num), 0)").
			Row().Scan(&max); err != nil {
			return err
		}

		var ids []uint
		if err := gdb.Model(&model.Basket{}).
			Where("multi_distrib_id = ? AND num = 0", mdID).
			Order("created_at, id").
			Pluck("id", &ids).Error; err != nil {
			return err
		}

		for _, id := range ids {
			max++
			if err := gdb.Model(&model.Basket{}).
				Where("id = ?", id).
				Update("num", max).Error; err != nil {
				return err
			}
			total++
		}
	}
	if total > 0 {
		log.Printf("BackfillBasketNumbers: %d paniers numérotés", total)
	}
	return nil
}

// enforceBasketUniqueness pose les deux unicités du panier : une par adhérent
// et par distribution, un numéro par distribution. Sans elles, deux commandes
// simultanées pourraient encore créer deux paniers au même adhérent.
func enforceBasketUniqueness(gdb *gorm.DB) error {
	index := []struct {
		nom string
		sql string
	}{
		{"idx_baskets_user_per_distrib",
			"CREATE UNIQUE INDEX idx_baskets_user_per_distrib ON baskets (user_id, multi_distrib_id)"},
		{"idx_baskets_num_per_distrib",
			"CREATE UNIQUE INDEX idx_baskets_num_per_distrib ON baskets (multi_distrib_id, num)"},
	}
	for _, idx := range index {
		if gdb.Migrator().HasIndex(&model.Basket{}, idx.nom) {
			continue
		}
		if err := gdb.Exec(idx.sql).Error; err != nil {
			// Un index refusé signale des doublons que les étapes précédentes
			// n'ont pas su réduire : on le dit sans empêcher le démarrage.
			log.Printf("BackfillBasketNumbers: index %s non créé: %v", idx.nom, err)
		}
	}
	return nil
}
