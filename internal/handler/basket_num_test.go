package handler

import (
	"testing"
	"time"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// seedMultiDistrib crée une distribution vide et programme son effacement.
func seedMultiDistrib(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	md := model.MultiDistrib{
		GroupID:          9900,
		PlaceID:          9900,
		DistribStartDate: time.Now(),
		DistribEndDate:   time.Now().Add(2 * time.Hour),
	}
	if err := db.Create(&md).Error; err != nil {
		t.Fatalf("distribution : %v", err)
	}
	t.Cleanup(func() {
		var distribs []uint
		db.Model(&model.Distribution{}).Where("multi_distrib_id = ?", md.ID).Pluck("id", &distribs)
		if len(distribs) > 0 {
			db.Where("distribution_id IN ?", distribs).Delete(&model.UserOrder{})
		}
		db.Where("multi_distrib_id = ?", md.ID).Delete(&model.Basket{})
		db.Where("multi_distrib_id = ?", md.ID).Delete(&model.Distribution{})
		db.Delete(&model.MultiDistrib{}, md.ID)
	})
	return md.ID
}

// seedDistribution ajoute la livraison d'un producteur à cette distribution.
func seedDistribution(t *testing.T, db *gorm.DB, multiDistribID, catalogID uint) uint {
	t.Helper()
	d := model.Distribution{CatalogID: catalogID, MultiDistribID: multiDistribID}
	if err := db.Create(&d).Error; err != nil {
		t.Fatalf("livraison : %v", err)
	}
	return d.ID
}

// seedOrder passe une commande par le chemin le plus brut qui soit — un Create
// direct, sans le service. C'est celui qu'empruntent la saisie par un
// responsable et l'API de compatibilité, et celui qui n'attachait aucun panier.
func seedOrder(t *testing.T, db *gorm.DB, distribID, userID uint) model.UserOrder {
	t.Helper()
	o := model.UserOrder{
		UserID:         userID,
		ProductID:      9900,
		Quantity:       1,
		ProductPrice:   1,
		DistributionID: &distribID,
	}
	if err := db.Create(&o).Error; err != nil {
		t.Fatalf("commande : %v", err)
	}
	return o
}

// Le numéro de panier identifie l'adhérent pendant toute la distribution : il
// doit être le même sur la liste de chaque producteur présent ce jour-là.
//
// Il ne l'était plus. Le numéro était un rang recalculé par écran, sur les
// seules commandes affichées ; filtré par producteur, l'écran renumérotait à
// partir de 1 le sous-ensemble qu'il montrait, et le même adhérent portait un
// numéro chez l'un, un autre chez le suivant.
func TestBasketNumberIsSharedAcrossVendors(t *testing.T) {
	db := testDB(t)

	mdID := seedMultiDistrib(t, db)
	chezA := seedDistribution(t, db, mdID, 9901)
	chezB := seedDistribution(t, db, mdID, 9902)

	// Trois adhérents qui ne se recouvrent pas d'un producteur à l'autre :
	// c'est ce décalage qui faisait diverger les rangs calculés.
	const (
		premier = 99001 // commande chez A, puis chez B
		second  = 99002 // chez B seulement
		dernier = 99003 // chez A seulement
	)
	seedOrder(t, db, chezA, premier)
	seedOrder(t, db, chezB, second)
	seedOrder(t, db, chezB, premier)
	seedOrder(t, db, chezA, dernier)

	nums := basketNumbers(db, mdID)

	for _, uid := range []uint{premier, second, dernier} {
		if nums[uid] == 0 {
			t.Fatalf("l'adhérent %d n'a pas de numéro : sa commande n'a pas créé de panier", uid)
		}
	}
	if nums[premier] == nums[second] || nums[premier] == nums[dernier] || nums[second] == nums[dernier] {
		t.Fatalf("deux adhérents partagent un numéro : %v", nums)
	}

	// Le numéro doit être celui de la distribution entière. S'il dépendait des
	// commandes d'un producteur, celui qui n'a commandé que chez B — le second
	// à commander chez lui — porterait le rang 2 au lieu de son rang réel.
	if nums[second] != 2 {
		t.Errorf("le second adhérent porte le numéro %d, attendu 2 : le numéro suit le producteur et non la distribution (%v)", nums[second], nums)
	}
	if nums[premier] != 1 || nums[dernier] != 3 {
		t.Errorf("les numéros ne suivent pas l'ordre des premières commandes : %v", nums)
	}
}

// Commander chez un second producteur ne donne pas un second panier : c'est ce
// qui rend le numéro commun à tous les producteurs.
func TestSecondVendorReusesTheSameBasket(t *testing.T) {
	db := testDB(t)

	mdID := seedMultiDistrib(t, db)
	chezA := seedDistribution(t, db, mdID, 9903)
	chezB := seedDistribution(t, db, mdID, 9904)
	const adherent = 99010

	premiere := seedOrder(t, db, chezA, adherent)
	seconde := seedOrder(t, db, chezB, adherent)

	if premiere.BasketID == nil || seconde.BasketID == nil {
		t.Fatal("une commande a été créée sans panier")
	}
	if *premiere.BasketID != *seconde.BasketID {
		t.Fatalf("deux paniers pour un même adhérent (%d et %d) : il porterait deux numéros",
			*premiere.BasketID, *seconde.BasketID)
	}

	var paniers int64
	db.Model(&model.Basket{}).Where("multi_distrib_id = ? AND user_id = ?", mdID, adherent).Count(&paniers)
	if paniers != 1 {
		t.Fatalf("%d paniers en base pour un adhérent et une distribution, attendu 1", paniers)
	}
}

// Une commande sans distribution ne peut pas être rattachée à un panier — la
// distribution est ce qui désigne le jour. Elle doit néanmoins se créer, sans
// quoi le hook ferait échouer un cas que l'application accepte.
func TestOrderWithoutDistributionStillCreated(t *testing.T) {
	db := testDB(t)

	o := model.UserOrder{UserID: 99020, ProductID: 9900, Quantity: 1, ProductPrice: 1}
	if err := db.Create(&o).Error; err != nil {
		t.Fatalf("commande sans distribution refusée : %v", err)
	}
	t.Cleanup(func() { db.Delete(&model.UserOrder{}, o.ID) })

	if o.BasketID != nil {
		t.Error("un panier a été attaché alors qu'aucune distribution ne le situe")
	}
}

// Les trois gabarits qui affichent le numéro doivent rester analysables : la
// condition qui masque un numéro absent — un panier d'avant la migration — y a
// été ajoutée à la main, et un {{end}} oublié ne se verrait qu'à l'affichage.
func TestBasketNumberTemplatesParse(t *testing.T) {
	gabarits := [][]string{
		{"contractadmin_orders_by_date.html"},
		{"emargement_print.html"},
		{"base.html", "design.html", "contractadmin_layout.html", "contractadmin_orders.html"},
	}
	for _, noms := range gabarits {
		if _, err := loadTemplatesFromRoot(t, noms...); err != nil {
			t.Errorf("%v : %v", noms, err)
		}
	}
}
