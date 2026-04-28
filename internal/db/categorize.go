package db

import (
	"strings"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// AutoCategorizeProducts assigne une TxpSubCategory aux produits qui n'en ont
// pas, en se basant sur des mots-clés présents dans le nom du produit.
//
// Idempotent : ignore les produits déjà classés (TxpSubCategoryID NOT NULL).
// Les produits dont le nom ne matche aucun mot-clé restent sans catégorie ; le
// shop les place dans le bucket "Autres / Tous" via le fallback du handler.
//
// Retourne le nombre de produits effectivement mis à jour.
func AutoCategorizeProducts(db *gorm.DB) (int, error) {
	// Récupère les sous-catégories "Tous" de chaque catégorie taxonomique,
	// indexées par l'image de la catégorie parente (clé stable et lisible).
	var cats []model.TxpCategory
	if err := db.Preload("SubCategories").Find(&cats).Error; err != nil {
		return 0, err
	}
	subByImage := make(map[string]uint, len(cats))
	for _, c := range cats {
		if len(c.SubCategories) > 0 {
			subByImage[c.Image] = c.SubCategories[0].ID
		}
	}

	rules := categorizationRules()

	var products []model.Product
	if err := db.Where("txp_sub_category_id IS NULL").Find(&products).Error; err != nil {
		return 0, err
	}

	updated := 0
	for _, p := range products {
		image := guessCategoryImage(p.Name, rules)
		if image == "" {
			continue
		}
		subID, ok := subByImage[image]
		if !ok {
			continue
		}
		if err := db.Model(&model.Product{}).
			Where("id = ?", p.ID).
			Update("txp_sub_category_id", subID).Error; err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

// guessCategoryImage retourne l'image (basename) de la TxpCategory matchée,
// ou "" si aucune règle ne s'applique.
func guessCategoryImage(name string, rules []categorizationRule) string {
	n := normalizeForKeyword(name)
	for _, r := range rules {
		for _, kw := range r.keywords {
			if strings.Contains(n, kw) {
				return r.image
			}
		}
	}
	return ""
}

type categorizationRule struct {
	image    string // doit matcher TxpCategory.Image (ex "viande-charcuterie")
	keywords []string
}

// categorizationRules retourne les règles de matching, dans l'ordre où elles
// sont évaluées (première qui matche gagne).
//
// Ordre soigné pour éviter les faux positifs courants :
//   - Variétés ambiguës en haute priorité (ex "moutarde rouge" est un légume,
//     doit gagner sur "moutarde" en épicerie)
//   - Catégories de produits spécifiques (mer, viande, crémerie, boulangerie)
//   - Épicerie AVANT boissons et fruits-légumes : "vinaigre" gagne sur
//     "cidre" (boisson) et "orange" (fruit)
//   - Boissons AVANT fruits-légumes : "jus de pomme" gagne sur "pomme"
//   - Fruits-légumes générique en dernier
func categorizationRules() []categorizationRule {
	return []categorizationRule{
		// Variétés de feuilles asiatiques avec des noms qui collisionnent
		// avec des keywords plus généraux d'épicerie ("moutarde").
		{"fruits-legumes", []string{
			"moutarde rouge", "moutarde japonaise", "moutarde feuille",
		}},
		{"produits-mer", []string{
			"saumon", "thon", "sardine", "maquereau", "cabillaud", "lieu noir",
			"truite", "crevette", "moule", "huitre", "hareng", "anchois",
			"poisson", "fruits de mer",
		}},
		{"viande-charcuterie", []string{
			"poulet", "boeuf", "porc", "agneau", "veau", "dinde", "pintade",
			"lapin", "canard", "magret", "cuisse", "saucisse", "saucisson",
			"jambon", "lardon", "rillette", "pate", "terrine", "andouille",
			"boudin", "foie", "gesier", "abats", "oie", "viande", "charcuterie",
			"steak", "rosbif", "escalope", "cote de", "filet mignon", "rumsteck",
			"merguez", "chipolata", "tripes", "volaille", "volalle", "burger",
			"carcasse",
		}},
		{"cremerie", []string{
			"fromage", "yaourt", "beurre", "lait", "creme", "faisselle",
			"comte", "brie", "camembert", "chevre", "mimolette", "gruyere",
			"emmental", "raclette", "mozzarella", "ricotta", "feta", "tomme",
			"reblochon", "munster", "epoisses", "roquefort", "bleu",
			"morbier", "saint-nectaire", "cantal", "ossau", "oeuf",
			"pyramide", "cendre",
		}},
		{"boulangerie-patisserie", []string{
			"pain", "brioche", "baguette", "viennoiserie", "croissant",
			"galette", "fougasse", "biscuit", "gateau", "tarte", "patisserie",
			"cake", "cookie", "madeleine", "financier",
		}},
		{"epicerie", []string{
			"huile", "vinaigre", "miel", "confiture", "farine", "sucre",
			" sel ", "pates", "riz", "semoule", "quinoa", "cafe", " the ",
			"infusion", "conserve", "soupe", "chocolat", "cacao", "epice",
			"moutarde", "ketchup", "tisane", "herbes de provence",
		}},
		{"boissons", []string{
			"vin ", "biere", "jus de", "soda", " eau ", "sirop", "limonade",
			"kefir", "kombucha", "cidre", "hydromel", "liqueur", "champagne",
			"pastis", "rhum", " gin ", "vodka", "whisky",
		}},
		{"fruits-legumes", []string{
			"pomme", "poire", "carotte", "salade", "oignon", " ail ", "tomate",
			"courgette", "courge", "potiron", "poireau", "fraise", "framboise",
			"mure", "citron", "orange", "mandarine", "kiwi", "banane",
			"abricot", "peche", "prune", "raisin", "melon", "betterave",
			"radis", "epinard", "chou", "brocoli", "haricot", "fenouil",
			"navet", "panais", "topinambour", "mache", "roquette", "asperge",
			"aubergine", "poivron", "concombre", "fruit", "legume", "patate",
			"pomme de terre", "endive", "petit pois", "feve", "lentille",
			"aillet", "batavia", "blette", "cebette", "mizuna", "persil",
			"wasabino", "wasabi", "pdt", "rhubarbe", "cresson", "ciboulette",
			"basilic", "menthe", "thym", "romarin", "estragon", "choucroute",
		}},
		{"desserts-plats-prepares", []string{
			"glace", "sorbet", "mousse", "lasagne", "quiche", "ratatouille",
			"compote", "plat prepare", "creme dessert", "flan",
		}},
		{"hygiene", []string{
			"savon", "shampoing", "shampooing", "dentifrice", "lessive",
			"nettoyant", "gel douche", "deodorant",
		}},
	}
}

// normalizeForKeyword met le texte en minuscule, retire les accents les plus
// courants et entoure d'espaces pour faciliter les matches sur mots isolés
// (ex le mot-clé " ail " ne match pas "ailes").
func normalizeForKeyword(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte(' ')
	for _, r := range s {
		switch r {
		case 'à', 'â', 'ä', 'á', 'ã', 'å':
			b.WriteRune('a')
		case 'é', 'è', 'ê', 'ë':
			b.WriteRune('e')
		case 'î', 'ï', 'í', 'ì':
			b.WriteRune('i')
		case 'ô', 'ö', 'ó', 'ò':
			b.WriteRune('o')
		case 'ù', 'û', 'ü', 'ú':
			b.WriteRune('u')
		case 'ç':
			b.WriteRune('c')
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(' ')
	return b.String()
}
