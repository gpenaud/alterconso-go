package db

import (
	"os"
	"testing"
	"time"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDB ouvre la base indiquée par TEST_DSN.
//
// Les tests qui suivent ne se contentent pas d'y ajouter des lignes : ils
// défont les index d'unicité de « baskets » pour reconstituer l'état d'avant la
// migration, et BackfillBasketNumbers agit sur toute la table. Ils veulent donc
// une base dédiée aux tests, jamais celle d'un développement en cours.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN absent")
	}
	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	return gdb
}

// dropBasketIndexes remet la table dans l'état qui précédait la migration, et
// garantit que les index seront rétablis même si le test échoue en chemin.
func dropBasketIndexes(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	noms := []string{"idx_baskets_user_per_distrib", "idx_baskets_num_per_distrib"}
	for _, nom := range noms {
		gdb.Exec("DROP INDEX " + nom + " ON baskets")
	}
	t.Cleanup(func() {
		if err := enforceBasketUniqueness(gdb); err != nil {
			t.Errorf("index non rétablis : %v", err)
		}
	})
}

// seedDistributionWithOrders monte une distribution à deux producteurs, puis
// efface les paniers que le hook vient de créer : on obtient les données telles
// que la réécriture les laissait — des commandes sans panier, donc sans numéro.
func seedDistributionWithOrders(t *testing.T, gdb *gorm.DB, adherents []uint) uint {
	t.Helper()
	md := model.MultiDistrib{
		GroupID:          9800,
		PlaceID:          9800,
		DistribStartDate: time.Now(),
		DistribEndDate:   time.Now().Add(2 * time.Hour),
	}
	if err := gdb.Create(&md).Error; err != nil {
		t.Fatalf("distribution : %v", err)
	}
	t.Cleanup(func() {
		var distribs []uint
		gdb.Model(&model.Distribution{}).Where("multi_distrib_id = ?", md.ID).Pluck("id", &distribs)
		if len(distribs) > 0 {
			gdb.Where("distribution_id IN ?", distribs).Delete(&model.UserOrder{})
		}
		gdb.Where("multi_distrib_id = ?", md.ID).Delete(&model.Basket{})
		gdb.Where("multi_distrib_id = ?", md.ID).Delete(&model.Distribution{})
		gdb.Delete(&model.MultiDistrib{}, md.ID)
	})

	livraisons := make([]uint, 0, 2)
	for _, catalogID := range []uint{9801, 9802} {
		d := model.Distribution{CatalogID: catalogID, MultiDistribID: md.ID}
		if err := gdb.Create(&d).Error; err != nil {
			t.Fatalf("livraison : %v", err)
		}
		livraisons = append(livraisons, d.ID)
	}

	// Les commandes sont espacées dans le temps : la numérotation doit suivre
	// l'ordre dans lequel les adhérents ont commandé.
	for i, uid := range adherents {
		for _, distribID := range livraisons {
			o := model.UserOrder{
				UserID:         uid,
				ProductID:      9800,
				Quantity:       1,
				ProductPrice:   1,
				DistributionID: &distribID,
				CreatedAt:      time.Now().Add(time.Duration(i) * time.Minute),
			}
			if err := gdb.Create(&o).Error; err != nil {
				t.Fatalf("commande : %v", err)
			}
		}
	}

	// Retour à l'état d'avant : plus de paniers, plus de rattachement.
	gdb.Where("multi_distrib_id = ?", md.ID).Delete(&model.Basket{})
	var distribs []uint
	gdb.Model(&model.Distribution{}).Where("multi_distrib_id = ?", md.ID).Pluck("id", &distribs)
	gdb.Model(&model.UserOrder{}).Where("distribution_id IN ?", distribs).
		Update("basket_id", nil)

	return md.ID
}

// Les commandes déjà en base ont été passées sans panier : la migration doit
// leur en donner un, numéroté, sans quoi les écrans n'auraient aucun numéro à
// afficher une fois le calcul par rang retiré.
func TestBackfillNumbersExistingOrders(t *testing.T) {
	gdb := testDB(t)
	dropBasketIndexes(t, gdb)

	adherents := []uint{98001, 98002, 98003}
	mdID := seedDistributionWithOrders(t, gdb, adherents)

	if err := BackfillBasketNumbers(gdb); err != nil {
		t.Fatalf("migration : %v", err)
	}

	var paniers []model.Basket
	gdb.Where("multi_distrib_id = ?", mdID).Order("num").Find(&paniers)
	if len(paniers) != len(adherents) {
		t.Fatalf("%d paniers créés pour %d adhérents", len(paniers), len(adherents))
	}
	for i, b := range paniers {
		if b.Num != i+1 {
			t.Errorf("numéros non contigus : panier de l'adhérent %d numéroté %d, attendu %d",
				b.UserID, b.Num, i+1)
		}
		if b.UserID != adherents[i] {
			t.Errorf("position %d : panier de l'adhérent %d, attendu %d — la numérotation ne suit pas l'ordre des commandes",
				i, b.UserID, adherents[i])
		}
	}

	// Les deux commandes d'un adhérent, une par producteur, pointent le même
	// panier : c'est ce qui lui donne un numéro unique chez les deux.
	var orphelines int64
	gdb.Raw(`
		SELECT COUNT(*) FROM user_orders uo
		JOIN distributions d ON d.id = uo.distribution_id
		WHERE d.multi_distrib_id = ? AND uo.basket_id IS NULL`, mdID).Scan(&orphelines)
	if orphelines != 0 {
		t.Errorf("%d commandes restées sans panier", orphelines)
	}
}

// Relancée à chaque démarrage, la migration ne doit rien changer une fois
// passée — surtout pas renuméroter des paniers que les adhérents connaissent.
func TestBackfillIsIdempotent(t *testing.T) {
	gdb := testDB(t)
	dropBasketIndexes(t, gdb)

	mdID := seedDistributionWithOrders(t, gdb, []uint{98011, 98012})
	if err := BackfillBasketNumbers(gdb); err != nil {
		t.Fatalf("première passe : %v", err)
	}

	avant := map[uint]int{}
	var paniers []model.Basket
	gdb.Where("multi_distrib_id = ?", mdID).Find(&paniers)
	for _, b := range paniers {
		avant[b.UserID] = b.Num
	}

	if err := BackfillBasketNumbers(gdb); err != nil {
		t.Fatalf("seconde passe : %v", err)
	}

	paniers = nil
	gdb.Where("multi_distrib_id = ?", mdID).Find(&paniers)
	if len(paniers) != len(avant) {
		t.Fatalf("%d paniers après la seconde passe, %d après la première", len(paniers), len(avant))
	}
	for _, b := range paniers {
		if avant[b.UserID] != b.Num {
			t.Errorf("l'adhérent %d est passé du numéro %d au numéro %d", b.UserID, avant[b.UserID], b.Num)
		}
	}
}

// Un adhérent qui aurait deux paniers sur la même distribution porterait deux
// numéros. La migration les fusionne avant de poser l'unicité qui l'interdira.
func TestBackfillMergesDuplicateBaskets(t *testing.T) {
	gdb := testDB(t)
	dropBasketIndexes(t, gdb)

	const adherent = 98021
	mdID := seedDistributionWithOrders(t, gdb, []uint{adherent})

	garde := model.Basket{UserID: adherent, MultiDistribID: mdID, Num: 1, CreatedAt: time.Now()}
	doublon := model.Basket{UserID: adherent, MultiDistribID: mdID, Num: 2, CreatedAt: time.Now()}
	if err := gdb.Create(&garde).Error; err != nil {
		t.Fatalf("panier : %v", err)
	}
	if err := gdb.Create(&doublon).Error; err != nil {
		t.Fatalf("panier en double : %v", err)
	}
	// Une commande rattachée au panier qui va disparaître : elle doit suivre.
	var distribID uint
	gdb.Model(&model.Distribution{}).Where("multi_distrib_id = ?", mdID).Limit(1).Pluck("id", &distribID)
	gdb.Model(&model.UserOrder{}).
		Where("distribution_id = ? AND user_id = ?", distribID, adherent).
		Update("basket_id", doublon.ID)

	if err := BackfillBasketNumbers(gdb); err != nil {
		t.Fatalf("migration : %v", err)
	}

	var restants []model.Basket
	gdb.Where("multi_distrib_id = ? AND user_id = ?", mdID, adherent).Find(&restants)
	if len(restants) != 1 {
		t.Fatalf("%d paniers après fusion, attendu 1", len(restants))
	}
	if restants[0].ID != garde.ID {
		t.Errorf("le panier conservé est le %d, attendu le plus ancien (%d)", restants[0].ID, garde.ID)
	}

	var perdues int64
	gdb.Model(&model.UserOrder{}).Where("basket_id = ?", doublon.ID).Count(&perdues)
	if perdues != 0 {
		t.Errorf("%d commandes pointent encore le panier supprimé", perdues)
	}
}

// Les commandes reprises de l'ancienne base portent un identifiant de panier
// qui ne désigne plus rien — les paniers, eux, n'ont pas été repris. Ces
// identifiants occupent la même plage que ceux des paniers que la migration
// recrée : chacun finit donc par désigner le panier d'un autre adhérent.
//
// Le cas est passé inaperçu jusqu'à une exécution sur des données réelles, où
// il touchait 15 887 des 16 444 commandes. Le test le fige.
func TestBackfillRepairsOrdersPointingAtAnotherMemberBasket(t *testing.T) {
	gdb := testDB(t)
	dropBasketIndexes(t, gdb)

	const (
		lui  = 98031
		elle = 98032
	)
	mdID := seedDistributionWithOrders(t, gdb, []uint{lui, elle})
	if err := BackfillBasketNumbers(gdb); err != nil {
		t.Fatalf("première passe : %v", err)
	}

	var panierDeElle model.Basket
	gdb.Where("multi_distrib_id = ? AND user_id = ?", mdID, elle).First(&panierDeElle)

	// On fait pointer les commandes de l'un vers le panier de l'autre : c'est
	// l'état exact que produisait la reprise des anciens identifiants.
	var distribs []uint
	gdb.Model(&model.Distribution{}).Where("multi_distrib_id = ?", mdID).Pluck("id", &distribs)
	gdb.Model(&model.UserOrder{}).
		Where("distribution_id IN ? AND user_id = ?", distribs, lui).
		Update("basket_id", panierDeElle.ID)

	if err := BackfillBasketNumbers(gdb); err != nil {
		t.Fatalf("seconde passe : %v", err)
	}

	var panierDeLui model.Basket
	gdb.Where("multi_distrib_id = ? AND user_id = ?", mdID, lui).First(&panierDeLui)

	var egarees int64
	gdb.Model(&model.UserOrder{}).
		Where("distribution_id IN ? AND user_id = ? AND basket_id <> ?", distribs, lui, panierDeLui.ID).
		Count(&egarees)
	if egarees != 0 {
		t.Errorf("%d commandes pointent encore le panier d'un autre adhérent", egarees)
	}
}
