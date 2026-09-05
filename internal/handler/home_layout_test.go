package handler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

// L'accueil met en avant ce sur quoi l'adhérent peut agir : la première
// distribution ouverte à la commande, et non la plus proche dans le temps —
// celle-ci peut être close depuis hier.
func TestHomeHighlightsTheOrderableDistribution(t *testing.T) {
	vues := []MultiDistribView{
		{ID: 1, Day: "05"},                 // fermée
		{ID: 2, Day: "12", CanOrder: true}, // ouverte
		{ID: 3, Day: "19"},
	}
	hero, suivantes := splitDistribs(vues)
	if hero == nil || hero.ID != 2 {
		t.Fatalf("la distribution ouverte devrait être mise en avant, obtenu %v", hero)
	}
	if len(suivantes) != 2 || suivantes[0].ID != 1 || suivantes[1].ID != 3 {
		t.Errorf("les autres doivent rester listées dans l'ordre, obtenu %v", suivantes)
	}
}

// Rien d'ouvert : la plus proche prend la place. L'écran ne doit jamais
// commencer par un vide.
func TestHomeFallsBackToTheNearest(t *testing.T) {
	hero, suivantes := splitDistribs([]MultiDistribView{{ID: 7}, {ID: 8}})
	if hero == nil || hero.ID != 7 {
		t.Fatalf("la plus proche devrait tenir la place, obtenu %v", hero)
	}
	if len(suivantes) != 1 || suivantes[0].ID != 8 {
		t.Errorf("obtenu %v", suivantes)
	}
}

// Aucune distribution : pas de mise en avant, et surtout pas de plantage.
func TestHomeWithoutDistributions(t *testing.T) {
	hero, suivantes := splitDistribs(nil)
	if hero != nil || len(suivantes) != 0 {
		t.Errorf("obtenu %v / %v", hero, suivantes)
	}
}

// L'accueil se rend jusqu'au bout, producteurs compris.
//
// Ce test existe pour une raison précise : le gabarit avait d'abord affiché un
// champ absent de cette branche, et le rendu échouait en silence — la page
// s'arrêtait au milieu, en répondant 200. Un gabarit qui ne parse plus, ou qui
// nomme un champ disparu, doit faire échouer la suite, pas la production.
func TestHomeRendersCompletely(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "cycles_style.html", "home.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	pd := PageData{
		User:     &model.User{ID: 1, FirstName: "Marie", LastName: "Dupont"},
		Group:    &model.Group{ID: 1, Name: "AMAP"},
		Category: "home",
	}
	pd.MultiDistribs = []MultiDistribView{
		{ID: 1, Day: "12", Month: "septembre", DayOfWeek: "jeudi",
			DayLabelFull: "Jeudi 12 septembre", Place: "Salle des fêtes",
			StartHour: "18:00", EndHour: "19:30", CanOrder: true, Distributions: true,
			Vendors: []VendorView{
				{ID: 1, Name: "Ferme du Pré", Organic: true, City: "Saint-Marc",
					Description: "Maraîchage sur sol vivant depuis 2011.",
					Products:    []ProductImageView{{URL: "/img/a.png", Name: "Carottes"}}},
				{ID: 2, Name: "Les Ruchers"}}},
		{ID: 2, Day: "19", Month: "septembre", DayOfWeek: "jeudi",
			Place: "Salle des fêtes", StartHour: "18:00", EndHour: "19:30",
			OrderNotYetOpen: true, OrderStartDate: "le 11/09"},
	}
	pd.HeroDistrib, pd.NextDistribs = splitDistribs(pd.MultiDistribs)

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", pd); err != nil {
		t.Fatalf("rendu interrompu : %v", err)
	}
	out := sb.String()

	for _, attendu := range []string{
		"ac-prochaine", "ac-pastille-date", "Salle des fêtes", "Ferme du Pré", "Les Ruchers",
		"2 producteurs", "Commander", "ac-suivantes", "Ouvre le 11/09", "</html>",
		// Le volet et son contenu : présentation, lieu, mention bio, produits.
		"ac-volet", "Maraîchage sur sol vivant", "Saint-Marc", "ac-bio", "Carottes",
	} {
		if !strings.Contains(out, attendu) {
			t.Errorf("%q manque au rendu", attendu)
		}
	}
}

// Sans distribution, l'écran le dit et se rend quand même.
func TestHomeRendersWhenEmpty(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "cycles_style.html", "home.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}
	var sb strings.Builder
	err = tpl.ExecuteTemplate(&sb, "base", PageData{
		User:  &model.User{ID: 1, FirstName: "Marie"},
		Group: &model.Group{ID: 1, Name: "AMAP"}, Category: "home",
	})
	if err != nil {
		t.Fatalf("rendu interrompu : %v", err)
	}
	if !strings.Contains(sb.String(), "Aucune distribution prévue") {
		t.Error("l'absence de distribution devrait se dire")
	}
}

// Le fragment du défilement continu rend les mêmes lignes que l'accueil, sans
// la page autour : c'est le même sous-gabarit, donc les distributions ajoutées
// en défilant se déplient comme les premières.
func TestScrollFragmentSharesTheSameRows(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "home.html", "home_more.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	pd := PageData{}
	pd.MultiDistribs = []MultiDistribView{
		{ID: 5, Day: "26", Month: "septembre", DayOfWeek: "jeudi", Place: "Salle",
			StartHour: "18:00", EndHour: "19:30", Distributions: true,
			Vendors: []VendorView{{ID: 1, Name: "Ferme du Pré"}}},
	}

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "fragment", pd); err != nil {
		t.Fatalf("rendu : %v", err)
	}
	out := sb.String()

	// Le contenu dépliable est là…
	for _, attendu := range []string{"ac-suivante-bloc", "ac-suivante-panneau",
		"Ferme du Pré", "jeudi 26 septembre"} {
		if !strings.Contains(out, attendu) {
			t.Errorf("%q manque au fragment", attendu)
		}
	}
	// …et rien de la page autour : l'insérer dans la liste doublerait sinon
	// l'en-tête et le pied à chaque défilement.
	for _, interdit := range []string{"<html", "<body", "ac-lateral", "<footer"} {
		if strings.Contains(out, interdit) {
			t.Errorf("le fragment ne doit pas contenir %q", interdit)
		}
	}
}

// Le volet « producteurs » a la même structure dans la carte mise en avant et
// dans les distributions repliées : c'est le même sous-gabarit, et il est
// fermé au rendu — on l'ouvre, il ne s'impose pas.
func TestVendorPanelIsSharedAndClosedByDefault(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "home.html", "home_more.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	pd := PageData{}
	pd.MultiDistribs = []MultiDistribView{{
		ID: 5, Day: "26", Month: "septembre", DayOfWeek: "jeudi", Place: "Salle",
		StartHour: "18:00", EndHour: "19:30", Distributions: true,
		Vendors: []VendorView{{ID: 1, Name: "Ferme du Pré",
			Description: "Maraîchage sur sol vivant.",
			Products:    []ProductImageView{{URL: "/img/a.png", Name: "Carottes"}}}},
	}}

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "fragment", pd); err != nil {
		t.Fatalf("rendu : %v", err)
	}
	out := sb.String()

	// Une distribution chargée au défilement porte le même volet, complet.
	for _, attendu := range []string{"ac-volet", "ac-producteur", "Maraîchage sur sol vivant",
		"Carottes", `aria-expanded="false"`} {
		if !strings.Contains(out, attendu) {
			t.Errorf("%q manque à la distribution repliée", attendu)
		}
	}
	// Fermé au rendu : la classe « ouvert » ne s'y trouve pas.
	if strings.Contains(out, "ac-volet ouvert") {
		t.Error("le volet devrait être replié par défaut")
	}
}

// La bande de vignettes alterne les producteurs.
//
// Elle prenait les premiers produits rencontrés, donc presque tous du même
// catalogue : elle montrait une ferme et taisait les autres, alors qu'elle est
// là pour dire ce qu'on trouvera.
func TestProductStripAlternatesVendors(t *testing.T) {
	// Des noms distincts : la bande écarte les conditionnements d'un même
	// produit, et vingt fois le même nom se réduirait à une seule vignette.
	produits := func(prefixe string, n int) []ProductImageView {
		var out []ProductImageView
		for i := 1; i <= n; i++ {
			out = append(out, ProductImageView{
				Name: fmt.Sprintf("%s produit %c", prefixe, rune('a'+i-1)),
				URL:  "/img/x.png", HasPhoto: true})
		}
		return out
	}

	// Un producteur très fourni, deux plus modestes : sans alternance, le
	// premier raflerait toute la bande.
	vendors := []VendorView{
		{ID: 1, Name: "A", Products: produits("A", 20)},
		{ID: 2, Name: "B", Products: produits("B", 2)},
		{ID: 3, Name: "C", Products: produits("C", 1)},
	}

	bande := pickAcrossVendors(vendors)
	if len(bande) != bandeTarget {
		t.Errorf("bande de %d vignettes, attendu %d", len(bande), bandeTarget)
	}
	vus := map[byte]int{}
	for _, p := range bande {
		vus[p.Name[0]]++
	}
	for _, nom := range []byte{'A', 'B', 'C'} {
		if vus[nom] == 0 {
			t.Errorf("le producteur %c n'apparaît pas dans la bande", nom)
		}
	}
	// Les trois premières vignettes viennent de trois producteurs différents.
	if bande[0].Name[0] == bande[1].Name[0] || bande[1].Name[0] == bande[2].Name[0] {
		t.Errorf("les premières vignettes devraient alterner, obtenu %s / %s / %s",
			bande[0].Name, bande[1].Name, bande[2].Name)
	}
}

// Plus de producteurs que la cible : la bande s'élargit pour n'en éclipser
// aucun, sans dépasser la limite au-delà de laquelle les vignettes deviennent
// illisibles.
func TestProductStripGrowsForManyVendors(t *testing.T) {
	var beaucoup []VendorView
	for i := 0; i < 15; i++ {
		beaucoup = append(beaucoup, VendorView{ID: uint(i + 1),
			Products: []ProductImageView{{Name: fmt.Sprintf("produit %d", i),
				URL: "/img/x.png", HasPhoto: true}}})
	}
	if n := len(pickAcrossVendors(beaucoup)); n != bandeMax {
		t.Errorf("bande de %d, attendu la limite de %d", n, bandeMax)
	}

	// Dix producteurs d'un produit chacun : tous montrés, aucun doublon.
	var dix []VendorView
	for i := 0; i < 10; i++ {
		dix = append(dix, VendorView{ID: uint(i + 1),
			Products: []ProductImageView{{Name: fmt.Sprintf("produit %d", i),
				URL: "/img/x.png", HasPhoto: true}}})
	}
	if n := len(pickAcrossVendors(dix)); n != 10 {
		t.Errorf("bande de %d, attendu 10 — un par producteur", n)
	}
}

// Peu de produits : la bande se contente de ce qui existe, sans boucler.
func TestProductStripStopsWhenExhausted(t *testing.T) {
	bande := pickAcrossVendors([]VendorView{
		{ID: 1, Products: []ProductImageView{{Name: "a", HasPhoto: true}}},
		{ID: 2, Products: []ProductImageView{{Name: "b", HasPhoto: true}}},
	})
	if len(bande) != 2 {
		t.Errorf("obtenu %d vignettes, attendu 2", len(bande))
	}
	if len(pickAcrossVendors(nil)) != 0 {
		t.Error("sans producteur, la bande est vide")
	}
}

// La bande n'accueille que de vraies photos. Un produit sans image reçoit une
// illustration générique, qui ne dit rien de ce qu'on trouvera : une rangée de
// silhouettes grises dessert la distribution qu'elle annonce.
func TestProductStripKeepsOnlyRealPhotos(t *testing.T) {
	vendors := []VendorView{
		{ID: 1, Name: "Illustré", Products: []ProductImageView{
			{Name: "avec", URL: "/img/vrai.png", HasPhoto: true},
			{Name: "sans", URL: "/img/taxo/grey/fruits-legumes.png"},
		}},
		{ID: 2, Name: "Sans photo", Products: []ProductImageView{
			{Name: "rien", URL: "/img/taxo/grey/fruits-legumes.png"},
		}},
	}

	bande := pickAcrossVendors(vendors)
	if len(bande) != 1 {
		t.Fatalf("une seule vraie photo attendue, obtenu %d", len(bande))
	}
	if bande[0].Name != "avec" {
		t.Errorf("la vignette retenue devrait être la photo réelle, obtenu %q", bande[0].Name)
	}

	// Aucune vraie photo : pas de bande du tout, plutôt qu'une rangée grise.
	if n := len(pickAcrossVendors([]VendorView{
		{ID: 1, Products: []ProductImageView{{Name: "rien", URL: "/img/taxo/grey/x.png"}}},
	})); n != 0 {
		t.Errorf("sans photo réelle la bande doit rester vide, obtenu %d", n)
	}
}

// Un producteur qui vend plusieurs sortes de choses doit les montrer toutes.
//
// La Ferme du Jointout a des fromages et cent cinquante légumes : ses vignettes
// venaient du même rayon, et la bande taisait la crémerie.
func TestVendorProductsSpreadAcrossCategories(t *testing.T) {
	const (
		legumes = uint(1)
		fromage = uint(2)
	)
	var produits []ProductImageView
	for i := 0; i < 10; i++ {
		produits = append(produits, ProductImageView{Name: "légume", Category: legumes, HasPhoto: true})
	}
	produits = append(produits,
		ProductImageView{Name: "fromage", Category: fromage, HasPhoto: true},
		ProductImageView{Name: "fromage", Category: fromage, HasPhoto: true})

	ordonnes := spreadCategories(produits)
	if len(ordonnes) != len(produits) {
		t.Fatalf("aucun produit ne doit disparaître : %d au lieu de %d", len(ordonnes), len(produits))
	}
	// Le fromage ne doit plus attendre la onzième place.
	if ordonnes[1].Category != fromage {
		t.Errorf("la deuxième vignette devrait changer de rayon, obtenu la catégorie %d",
			ordonnes[1].Category)
	}
	// Les deux premières vignettes couvrent deux rayons.
	if ordonnes[0].Category == ordonnes[1].Category {
		t.Error("les premières vignettes devraient alterner les catégories")
	}
}

// Un seul rayon, ou trop peu de produits : l'ordre d'origine est conservé,
// mieux vaut ne rien brasser sans raison.
func TestSpreadLeavesSingleCategoryAlone(t *testing.T) {
	un := []ProductImageView{{Name: "a", Category: 1}, {Name: "b", Category: 1}, {Name: "c", Category: 1}}
	got := spreadCategories(un)
	for i := range un {
		if got[i].Name != un[i].Name {
			t.Fatalf("l'ordre ne devrait pas changer : %v", got)
		}
	}
	court := []ProductImageView{{Name: "a", Category: 1}, {Name: "b", Category: 2}}
	if len(spreadCategories(court)) != 2 {
		t.Error("une liste courte doit rester intacte")
	}
}

// La pastille de date donne à la carte son point d'ancrage, et prend la
// couleur de l'état : verte quand on peut commander, neutre sinon.
func TestDateBadgeCarriesTheState(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "cycles_style.html", "home.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	rendu := func(peutCommander bool) string {
		pd := PageData{
			User:  &model.User{ID: 1, FirstName: "Marie"},
			Group: &model.Group{ID: 1, Name: "AMAP"}, Category: "home",
		}
		pd.MultiDistribs = []MultiDistribView{{
			ID: 1, Day: "12", Month: "septembre", DayOfWeek: "jeudi",
			DayLabelFull: "Jeudi 12 septembre", Place: "Salle",
			StartHour: "18:00", EndHour: "19:30",
			CanOrder: peutCommander, Distributions: true,
		}}
		pd.HeroDistrib, pd.NextDistribs = splitDistribs(pd.MultiDistribs)
		var sb strings.Builder
		if err := tpl.ExecuteTemplate(&sb, "base", pd); err != nil {
			t.Fatalf("rendu : %v", err)
		}
		return sb.String()
	}

	// La date vit dans la pastille, le titre porte le lieu : la répéter aux
	// deux endroits prenait la place de ce qui manquait.
	ouvert := rendu(true)
	for _, attendu := range []string{"ac-pastille-date ouverte", ">jeudi<", ">12<",
		">septembre<", "Commandes ouvertes", "<h2 class=\"ac-prochaine-date\">Salle</h2>"} {
		if !strings.Contains(ouvert, attendu) {
			t.Errorf("%q manque quand les commandes sont ouvertes", attendu)
		}
	}

	ferme := rendu(false)
	if strings.Contains(ferme, "ac-pastille-date ouverte") {
		t.Error("la pastille ne doit pas s'afficher ouverte quand les commandes sont closes")
	}
	if !strings.Contains(ferme, "ac-pastille-date ") {
		t.Error("la pastille doit rester présente, dans son état neutre")
	}
}

// Les permanences se disent par une pastille sous le lieu : rouge si personne
// ne s'est inscrit, jaune s'il en manque, verte si tout est tenu. Le compte y
// figure en toutes lettres — la couleur seule se perd au soleil, sur un écran
// mal réglé, ou pour qui distingue mal le rouge du vert.
func TestVolunteerBadgeShowsThreeStates(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "cycles_style.html", "home.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	rendu := func(total, filled int, moi bool) string {
		pd := PageData{
			User:  &model.User{ID: 1, FirstName: "Guillaume"},
			Group: &model.Group{ID: 1, Name: "AMAP"}, Category: "home",
		}
		pd.MultiDistribs = []MultiDistribView{{
			ID: 1, Day: "12", Month: "septembre", DayOfWeek: "jeudi",
			Place: "Salle", StartHour: "18:00", EndHour: "19:30", Distributions: true,
			VolunteerTotal: total, VolunteerFilled: filled, UserIsVolunteer: moi,
			VolunteerFrom: "2026-09-06", VolunteerTo: "2026-09-13",
		}}
		pd.HeroDistrib, pd.NextDistribs = splitDistribs(pd.MultiDistribs)
		var sb strings.Builder
		if err := tpl.ExecuteTemplate(&sb, "base", pd); err != nil {
			t.Fatalf("rendu : %v", err)
		}
		return sb.String()
	}

	cases := []struct {
		nom           string
		total, filled int
		etat, compte  string
	}{
		{"personne", 2, 0, "ac-permanences vide", "Permanences 0/2"},
		{"un sur deux", 2, 1, "ac-permanences partiel", "Permanences 1/2"},
		{"au complet", 2, 2, "ac-permanences complet", "Permanences 2/2"},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			out := rendu(tc.total, tc.filled, false)
			if !strings.Contains(out, tc.etat) {
				t.Errorf("état %q attendu", tc.etat)
			}
			if !strings.Contains(out, tc.compte) {
				t.Errorf("le compte %q devrait figurer", tc.compte)
			}
			// Plus aucune barre : tout se joue sur la pastille.
			if strings.Contains(out, "ac-appel-benevoles") {
				t.Error("le bandeau des bénévoles ne doit plus paraître")
			}
			// Et elle mène à la page d'inscription.
			// Et elle ouvre le calendrier sur la semaine de cette
			// distribution, non sur la semaine courante — qui peut être à des
			// mois de là.
			if !strings.Contains(out, "/distribution/volunteersCalendar?from=2026-09-06&amp;to=2026-09-13") {
				t.Error("la pastille devrait ouvrir la bonne semaine")
			}
		})
	}

	// Inscrit : la pastille le signale, sans rien réclamer de plus.
	if !strings.Contains(rendu(2, 1, true), "· vous") {
		t.Error("la pastille devrait signaler que le lecteur tient une permanence")
	}

	// Aucun rôle défini : pas de pastille du tout. On vise l'attribut et non
	// le seul nom de classe : le script de la page cite ce sélecteur pour
	// retenir d'où l'on part vers les permanences, et le chercher nu ferait
	// répondre le script à la place du balisage.
	if strings.Contains(rendu(0, 0, false), `class="ac-permanences`) {
		t.Error("sans poste à tenir, aucune pastille ne doit paraître")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// La mise en avant vient du catalogue et non du producteur : elle doit donc
// remonter jusqu'à la journée, sans se dédoubler quand plusieurs catalogues
// en portent une.
func TestHighlightDesDistribs(t *testing.T) {
	label := func(s string) *string { return &s }
	cat := func(h *string) model.Distribution {
		return model.Distribution{Catalog: model.Catalog{HighlightLabel: h}}
	}

	if got := highlightDesDistribs(nil); got != "" {
		t.Errorf("aucune distribution : %q, attendu vide", got)
	}
	if got := highlightDesDistribs([]model.Distribution{cat(nil), cat(nil)}); got != "" {
		t.Errorf("aucun catalogue mis en avant : %q, attendu vide", got)
	}
	if got := highlightDesDistribs([]model.Distribution{
		cat(nil), cat(label("Agrumes de Sicile")), cat(label("Miel")),
	}); got != "Agrumes de Sicile" {
		t.Errorf("deux campagnes : %q, attendu la première seule", got)
	}
	// Un libellé qui n'est que des blancs ne met rien en avant : sinon la
	// pastille s'affiche vide, et on ne sait pas ce qu'elle annonce.
	if got := highlightDesDistribs([]model.Distribution{cat(label("   "))}); got != "" {
		t.Errorf("libellé blanc : %q, attendu vide", got)
	}
	if got := highlightDesDistribs([]model.Distribution{cat(label("  Miel  "))}); got != "Miel" {
		t.Errorf("libellé mal cadré : %q, attendu \"Miel\"", got)
	}
}

// Le producteur d'une campagne mise en avant ouvre le volet : l'annoncer puis
// le laisser en cinquième position reviendrait à le cacher.
func TestEpinglerEnTete(t *testing.T) {
	liste := func(ids ...uint) []VendorView {
		out := make([]VendorView, 0, len(ids))
		for _, id := range ids {
			out = append(out, VendorView{ID: id})
		}
		return out
	}
	ordre := func(vs []VendorView) string {
		out := ""
		for _, v := range vs {
			out += fmt.Sprintf("%d ", v.ID)
		}
		return strings.TrimSpace(out)
	}

	cas := []struct {
		nom    string
		in     []VendorView
		id     uint
		attend string
	}{
		{"remonte et garde l'ordre des autres", liste(1, 2, 3, 4), 3, "3 1 2 4"},
		{"déjà en tête", liste(7, 1, 2), 7, "7 1 2"},
		{"absent de la liste", liste(1, 2), 9, "1 2"},
		{"aucune mise en avant", liste(1, 2, 3), 0, "1 2 3"},
		{"un seul producteur", liste(5), 5, "5"},
	}
	for _, c := range cas {
		if got := ordre(epinglerEnTete(c.in, c.id)); got != c.attend {
			t.Errorf("%s : %q, attendu %q", c.nom, got, c.attend)
		}
	}

	// L'entrée ne doit pas être écrasée en place : la vue d'origine sert
	// encore à distribuer les vignettes.
	src := liste(1, 2, 3)
	epinglerEnTete(src, 3)
	if ordre(src) != "1 2 3" {
		t.Errorf("la liste d'origine a été remaniée : %q", ordre(src))
	}
}

// La mise en avant occupe une place que tout le groupe voit : c'est un
// arbitrage de calendrier. Un producteur qui tient son propre catalogue a le
// droit « catalogues » dessus — il ne doit pas pouvoir s'y mettre en avant.
func TestPeutMettreEnAvant(t *testing.T) {
	cas := []struct {
		nom    string
		pd     PageData
		attend bool
	}{
		{"responsable des distributions", PageData{HasDistributions: true}, true},
		{"gestionnaire du groupe", PageData{IsGroupManager: true, HasDistributions: true}, true},
		{"producteur avec son catalogue", PageData{HasCatalogAdmin: true}, false},
		{"responsable des membres", PageData{HasMembership: true}, false},
		{"adhérent sans fonction", PageData{}, false},
	}
	for _, c := range cas {
		if got := peutMettreEnAvant(c.pd); got != c.attend {
			t.Errorf("%s : %v, attendu %v", c.nom, got, c.attend)
		}
	}
}

// Le raccourci « Gérer mes produits » vise le producteur, pas celui qui
// administre : ce dernier a déjà l'écran dans son espace d'administration.
func TestEstProducteurAutonome(t *testing.T) {
	cas := []struct {
		nom    string
		pd     PageData
		attend bool
	}{
		{"producteur, un catalogue nommé",
			PageData{HasCatalogAdmin: true, AllowedCatalogIDs: []uint{2}}, true},
		{"producteur, deux catalogues",
			PageData{HasCatalogAdmin: true, AllowedCatalogIDs: []uint{2, 9}}, true},
		{"gestionnaire du groupe",
			PageData{IsGroupManager: true, HasCatalogAdmin: true}, false},
		{"responsable des distributions",
			PageData{HasCatalogAdmin: true, HasDistributions: true,
				AllowedCatalogIDs: []uint{2}}, false},
		{"droit catalogues global (aucune liste)",
			PageData{HasCatalogAdmin: true, AllowedCatalogIDs: nil}, false},
		{"adhérent sans fonction", PageData{}, false},
	}
	for _, c := range cas {
		if got := estProducteurAutonome(c.pd); got != c.attend {
			t.Errorf("%s : %v, attendu %v", c.nom, got, c.attend)
		}
	}
}

// Un seul catalogue mène droit aux produits : une liste d'un seul élément
// demanderait de choisir ce qu'il n'y a pas à choisir.
func TestLienMesProduits(t *testing.T) {
	cas := []struct {
		nom    string
		ids    []uint
		attend string
	}{
		{"un seul catalogue", []uint{7}, "/contractAdmin/products/7"},
		{"deux catalogues", []uint{7, 9}, "/contractAdmin"},
		{"aucun", nil, "/contractAdmin"},
	}
	for _, c := range cas {
		if got := lienMesProduits(c.ids); got != c.attend {
			t.Errorf("%s : %q, attendu %q", c.nom, got, c.attend)
		}
	}
}
