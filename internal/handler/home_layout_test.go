package handler

import "testing"

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

// La bande de vignettes alterne les producteurs.
//
// Elle prenait les premiers produits rencontrés, donc presque tous du même
// catalogue : elle montrait une ferme et taisait les autres, alors qu'elle est
// là pour dire ce qu'on trouvera.
func TestProductStripAlternatesVendors(t *testing.T) {
	produits := func(prefixe string, n int) []ProductImageView {
		var out []ProductImageView
		for i := 1; i <= n; i++ {
			out = append(out, ProductImageView{Name: prefixe, URL: "/img/x.png"})
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
	vus := map[string]int{}
	for _, p := range bande {
		vus[p.Name]++
	}
	for _, nom := range []string{"A", "B", "C"} {
		if vus[nom] == 0 {
			t.Errorf("le producteur %s n'apparaît pas dans la bande", nom)
		}
	}
	// Les trois premières vignettes viennent de trois producteurs différents.
	if bande[0].Name == bande[1].Name || bande[1].Name == bande[2].Name {
		t.Errorf("les premières vignettes devraient alterner, obtenu %s %s %s",
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
			Products: []ProductImageView{{Name: "p", URL: "/img/x.png"}}})
	}
	if n := len(pickAcrossVendors(beaucoup)); n != bandeMax {
		t.Errorf("bande de %d, attendu la limite de %d", n, bandeMax)
	}

	// Dix producteurs d'un produit chacun : tous montrés, aucun doublon.
	var dix []VendorView
	for i := 0; i < 10; i++ {
		dix = append(dix, VendorView{ID: uint(i + 1),
			Products: []ProductImageView{{Name: "p", URL: "/img/x.png"}}})
	}
	if n := len(pickAcrossVendors(dix)); n != 10 {
		t.Errorf("bande de %d, attendu 10 — un par producteur", n)
	}
}

// Peu de produits : la bande se contente de ce qui existe, sans boucler.
func TestProductStripStopsWhenExhausted(t *testing.T) {
	bande := pickAcrossVendors([]VendorView{
		{ID: 1, Products: []ProductImageView{{Name: "a"}}},
		{ID: 2, Products: []ProductImageView{{Name: "b"}}},
	})
	if len(bande) != 2 {
		t.Errorf("obtenu %d vignettes, attendu 2", len(bande))
	}
	if len(pickAcrossVendors(nil)) != 0 {
		t.Error("sans producteur, la bande est vide")
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

// L'appel aux bénévoles passe avant les producteurs : c'est ce qui demande une
// action. Et dès qu'on s'est inscrit, il remercie au lieu de réclamer.
func TestVolunteerCallComesFirstAndThanksOnceSignedUp(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "home.html", "home_more.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	rendu := func(v MultiDistribView) string {
		pd := PageData{}
		v.ID, v.Place, v.StartHour, v.EndHour = 5, "Salle", "18:00", "19:30"
		v.Vendors = []VendorView{{ID: 1, Name: "Ferme du Pré"}}
		pd.MultiDistribs = []MultiDistribView{v}
		var sb strings.Builder
		if err := tpl.ExecuteTemplate(&sb, "fragment", pd); err != nil {
			t.Fatalf("rendu : %v", err)
		}
		return sb.String()
	}

	// Il manque des bénévoles : bandeau d'alerte, avant le volet.
	manque := rendu(MultiDistribView{VolunteerNeeded: 2, VolunteerRoles: []string{"Accueil"}})
	iAppel := strings.Index(manque, "ac-appel-benevoles")
	iVolet := strings.Index(manque, "ac-volet")
	if iAppel < 0 || iVolet < 0 {
		t.Fatal("les deux blocs devraient être présents")
	}
	if iAppel > iVolet {
		t.Error("l'appel aux bénévoles devrait précéder le volet des producteurs")
	}
	if strings.Contains(manque, "ac-appel-benevoles inscrit") {
		t.Error("sans inscription, le bandeau ne doit pas remercier")
	}

	// Inscrit : le bandeau vire au vert et remercie, sans plus rien réclamer.
	inscrit := rendu(MultiDistribView{VolunteerNeeded: 2,
		UserIsVolunteer: true, UserVolunteerRole: "Accueil"})
	if !strings.Contains(inscrit, "ac-appel-benevoles inscrit") {
		t.Error("une fois inscrit, le bandeau devrait changer d'état")
	}
	if !strings.Contains(inscrit, "Merci") || !strings.Contains(inscrit, "Accueil") {
		t.Error("le remerciement devrait nommer la permanence tenue")
	}
	if strings.Contains(inscrit, "S'inscrire") {
		t.Error("on ne réclame pas deux fois à qui a déjà répondu")
	}

	// Rien à signaler : pas de bandeau du tout.
	if strings.Contains(rendu(MultiDistribView{}), "ac-appel-benevoles") {
		t.Error("sans besoin ni inscription, aucun bandeau ne doit paraître")
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

	ouvert := rendu(true)
	for _, attendu := range []string{"ac-pastille-date ouverte", ">jeudi<", ">12<",
		">septembre<", "Commandes ouvertes"} {
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
