package handler

import (
	"fmt"
	"html/template"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

// L'écran d'un adhérent se réduit à ce qu'il vient y faire : commander. Ni
// barre de navigation, ni icône d'administration — rien qui mène à un refus.
func TestPlainMemberSeesNothingAdministrative(t *testing.T) {
	out := renderNav(t, PageData{
		Group: &model.Group{ID: 1, Name: "AMAP"},
		User:  &model.User{ID: 9, FirstName: "Marie", LastName: "Dupont"},
	})

	for _, absent := range []string{`href="/amap"`, `href="/distribution"`,
		`href="/amapadmin"`, `href="/member"`, `href="/contractAdmin"`, `href="/admin"`} {
		if strings.Contains(out, absent) {
			t.Errorf("%s ne devrait pas s'afficher pour un adhérent", absent)
		}
	}
	// Ce qui lui appartient reste atteignable sous la pastille.
	for _, present := range []string{`href="/home"`, `href="/account"`,
		`href="/messages"`, `href="/user/logout"`} {
		if !strings.Contains(out, present) {
			t.Errorf("%s a disparu de la page", present)
		}
	}
	// La pastille porte ses initiales.
	if !strings.Contains(out, `<span class="ac-pastille">MD</span>`) {
		t.Error("la pastille devrait porter les initiales")
	}
}

// L'icône d'administration n'apparaît qu'à qui administre quelque chose — une
// seule délégation suffit à l'ouvrir.
func TestAdminIconAppearsOnlyWithRights(t *testing.T) {
	base := func() PageData {
		return PageData{Group: &model.Group{ID: 1, Name: "AMAP"},
			User: &model.User{ID: 9, FirstName: "Alix", LastName: "Viginier"}}
	}

	if strings.Contains(renderNav(t, base()), `href="/admin"`) {
		t.Error("un adhérent ne doit pas voir l'espace d'administration")
	}

	for nom, ajuste := range map[string]func(*PageData){
		"gestion des membres": func(p *PageData) { p.HasMembership = true },
		"distributions":       func(p *PageData) { p.HasDistributions = true },
		"catalogues":          func(p *PageData) { p.HasCatalogAdmin = true },
		"paramètres":          func(p *PageData) { p.HasParameters = true },
		"responsable":         func(p *PageData) { p.IsGroupManager = true },
	} {
		pd := base()
		ajuste(&pd)
		if !strings.Contains(renderNav(t, pd), `href="/admin"`) {
			t.Errorf("la délégation « %s » devrait ouvrir l'espace d'administration", nom)
		}
	}
}

// Chaque délégation ouvre son domaine dans l'espace, et lui seul : afficher une
// entrée dont le clic mènerait à un refus est une promesse qu'on ne tient pas.
func TestAdminSpaceFollowsDelegations(t *testing.T) {
	titres := func(pd PageData) []string {
		var out []string
		for _, t := range adminTilesFor(pd) {
			out = append(out, t.Title)
		}
		return out
	}
	contient := func(l []string, v string) bool {
		for _, s := range l {
			if s == v {
				return true
			}
		}
		return false
	}

	if n := len(adminTilesFor(PageData{})); n != 0 {
		t.Errorf("sans droit, l'espace ne montre rien, obtenu %d entrées", n)
	}

	distributions := titres(PageData{HasDistributions: true})
	if !contient(distributions, "Distributions") {
		t.Error("la délégation « distributions » doit ouvrir son domaine")
	}
	if contient(distributions, "Paramètres") || contient(distributions, "Membres") {
		t.Error("elle ne doit ouvrir que le sien")
	}

	parametres := titres(PageData{HasParameters: true})
	if !contient(parametres, "Paramètres") {
		t.Error("la délégation « paramètres » doit ouvrir son domaine")
	}
	if contient(parametres, "Distributions") {
		t.Error("elle ne doit pas ouvrir le calendrier")
	}

	// Le responsable de groupe a tout, sans qu'on ait à le lui accorder.
	chef := titres(PageData{IsGroupManager: true, HasDistributions: true,
		HasParameters: true, CanManageRights: true, HasDatabaseAdmin: true})
	for _, attendu := range []string{"Membres", "Distributions", "Catalogues",
		"Paramètres", "Droits", "Base de données"} {
		if !contient(chef, attendu) {
			t.Errorf("« %s » manque au responsable de groupe", attendu)
		}
	}
}

// L'écran des producteurs se rouvre à tout adhérent par configuration : un
// groupe qui s'en sert comme annuaire ne doit pas avoir à reprendre le code.
func TestVendorsReopenByConfiguration(t *testing.T) {
	if len(adminTilesFor(PageData{})) != 0 {
		t.Fatal("état de départ inattendu")
	}
	tiles := adminTilesFor(PageData{ShowVendorsTab: true})
	if len(tiles) != 1 || tiles[0].Title != "Producteurs" {
		t.Errorf("l'onglet rouvert devrait montrer les producteurs, obtenu %v", tiles)
	}
}

// chdirRepoRoot place le test a la racine du depot, d'ou loadTemplates lit
// templates/. Idempotente : plusieurs tests en ont besoin, et un second
// « ../.. » depuis la racine sortirait du depot.
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("templates"); err == nil {
		return
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("chdir : %v", err)
	}
}

// loadTemplatesFromRoot charge des gabarits depuis la racine du depot, ou
// qu'ait demarre le test.
func loadTemplatesFromRoot(t *testing.T, names ...string) (*template.Template, error) {
	t.Helper()
	chdirRepoRoot(t)
	return loadTemplates(names...)
}

func renderNav(t *testing.T, pd PageData) string {
	t.Helper()
	chdirRepoRoot(t)
	tpl, err := loadTemplates("base.html", "design.html", "home.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}
	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", pd); err != nil {
		t.Fatalf("render : %v", err)
	}
	return sb.String()
}

// Les écrans rendent des structures qui embarquent PageData — JoinRequestsData,
// CyclesData… Un gabarit commun qui appellerait une fonction avec « . » y
// recevrait la structure dérivée et non PageData : le rendu échoue alors en
// silence et tronque la page. Le menu latéral doit donc se composer depuis un
// champ, promu partout, et la page aller jusqu'à son terme.
func TestCommonLayoutRendersFullyForDerivedData(t *testing.T) {
	chdirRepoRoot(t)

	pd := PageData{
		User:     &model.User{ID: 1, FirstName: "Alix", LastName: "Viginier"},
		Group:    &model.Group{ID: 1, Name: "AMAP"},
		Category: "member",
	}
	pd.HasMembership = true
	pd.AdminTiles = adminTilesFor(pd)

	cas := map[string]struct {
		gabarit string
		donnees any
	}{
		"PageData nu":      {"member_requests.html", JoinRequestsData{PageData: pd}},
		"données de cycle": {"distribution_cycles.html", CyclesData{PageData: pd}},
	}

	for nom, tc := range cas {
		t.Run(nom, func(t *testing.T) {
			noms := []string{"base.html", "design.html", "cycles_style.html", tc.gabarit}
			tpl, err := loadTemplates(noms...)
			if err != nil {
				t.Fatalf("parse : %v", err)
			}
			var sb strings.Builder
			if err := tpl.ExecuteTemplate(&sb, "base", tc.donnees); err != nil {
				t.Fatalf("rendu interrompu : %v", err)
			}
			out := sb.String()
			for _, attendu := range []string{`id="ac-lateral"`, `id="ac-contenu"`, "</html>"} {
				if !strings.Contains(out, attendu) {
					t.Errorf("%s manque : la page est tronquée", attendu)
				}
			}
		})
	}
}

// La fiche publique d'un groupe se rend avec une structure qui n'a ni compte
// ni rubrique. Le gabarit commun doit s'en accommoder : « {{if .User}} » y
// interrompait le rendu, et la page s'arrêtait au milieu — pour des visiteurs
// qui n'étaient même pas connectés.
func TestCommonLayoutToleratesDataWithoutUser(t *testing.T) {
	chdirRepoRoot(t)
	tpl, err := loadTemplates("base.html", "group_public.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	var sb strings.Builder
	err = tpl.ExecuteTemplate(&sb, "base", GroupPublicData{
		Title: "Page publique",
		Group: &model.Group{ID: 1, Name: "AMAP"},
		Home:  "Bienvenue dans notre groupe.",
	})
	if err != nil {
		t.Fatalf("rendu interrompu : %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "</html>") {
		t.Error("la page publique devrait se rendre jusqu'au bout")
	}
	if !strings.Contains(out, "Bienvenue dans notre groupe") {
		t.Error("le mot du groupe devrait s'y afficher")
	}
}

// Le producteur qui ne tient que ses catalogues ne va pas dans l'espace
// d'administration : il n'y trouverait qu'un écran, celui que son raccourci
// lui donne déjà. L'en-tête lui montre donc le raccourci, et pas l'icône.
func TestRaccourciProducteurRemplaceLAdministration(t *testing.T) {
	pd := PageData{
		Group:             &model.Group{ID: 1, Name: "AMAP"},
		User:              &model.User{ID: 9, FirstName: "Alix", LastName: "Viginier"},
		HasCatalogAdmin:   true,
		AllowedCatalogIDs: []uint{7},
	}
	out := renderNav(t, pd)
	if strings.Contains(out, `href="/admin"`) {
		t.Error("un producteur ne doit pas voir l'espace d'administration")
	}
	if !strings.Contains(out, `href="/contractAdmin/products/7"`) {
		t.Error("son raccourci devrait mener droit à ses produits")
	}

	// Deux catalogues : le raccourci mène à la liste, il faut bien choisir.
	pd.AllowedCatalogIDs = []uint{7, 9}
	out = renderNav(t, pd)
	if !strings.Contains(out, `href="/contractAdmin"`) {
		t.Error("avec deux catalogues, le raccourci devrait mener à la liste")
	}

	// Une fonction transverse en plus, et l'espace d'administration reprend
	// ses droits : le raccourci n'a plus lieu d'être.
	pd.HasDistributions = true
	out = renderNav(t, pd)
	if !strings.Contains(out, `href="/admin"`) {
		t.Error("avec le droit distributions, l'espace devrait s'ouvrir")
	}
	if strings.Contains(out, `href="/contractAdmin/products/`) {
		t.Error("le raccourci vers ses produits ne devrait plus s'afficher")
	}
}

// Les icônes de l'interface doivent exister dans la fonte : une classe absente
// ne rend rien du tout, et rien à l'écran ne le signale.
func TestIconesConnuesDeLaFonte(t *testing.T) {
	css, err := os.ReadFile("../../www/font/icons.css")
	if err != nil {
		t.Skipf("fonte introuvable : %v", err)
	}
	dispo := map[string]bool{}
	for _, m := range regexp.MustCompile(`\.icon-([a-z0-9-]+):before`).FindAllStringSubmatch(string(css), -1) {
		dispo[m[1]] = true
	}
	if len(dispo) == 0 {
		t.Fatal("aucune icône lue dans la fonte")
	}

	// Les écrans refondus. Les gabarits hérités portent des classes absentes
	// de longue date ; les corriger est un autre chantier que celui-ci.
	for _, f := range []string{"design.html", "home.html", "cycles_style.html",
		"distribution_volunteers_calendar.html"} {
		src, err := os.ReadFile("../../templates/" + f)
		if err != nil {
			continue
		}
		for _, m := range regexp.MustCompile(`icon icon-([a-z0-9-]+)`).FindAllStringSubmatch(string(src), -1) {
			if !dispo[m[1]] {
				t.Errorf("%s : l'icône « %s » n'existe pas dans la fonte", f, m[1])
			}
		}
	}
}

// Le classement des bénévoles : par nom c'est un annuaire, par nombre de
// permanences c'est une répartition de l'effort.
func TestTrierParticipation(t *testing.T) {
	lignes := []VolParticipationRow{
		{UserID: 1, Name: "Zoé Martin", Done: 2},
		{UserID: 2, Name: "Alice Durand", Done: 5},
		{UserID: 3, Name: "Bruno Petit", Done: 0},
		{UserID: 4, Name: "Chloé Roux", Done: 2},
	}
	ordre := func(rs []VolParticipationRow) string {
		out := ""
		for _, r := range rs {
			out += fmt.Sprintf("%d ", r.UserID)
		}
		return strings.TrimSpace(out)
	}

	if got := ordre(trierParticipation(lignes, "nom")); got != "2 3 4 1" {
		t.Errorf("par nom : %q, attendu \"2 3 4 1\"", got)
	}
	// Ex æquo à 2 permanences : Chloé avant Zoé, par le nom.
	if got := ordre(trierParticipation(lignes, "plus")); got != "2 4 1 3" {
		t.Errorf("du plus actif : %q, attendu \"2 4 1 3\"", got)
	}
	if got := ordre(trierParticipation(lignes, "moins")); got != "3 4 1 2" {
		t.Errorf("du moins actif : %q, attendu \"3 4 1 2\"", got)
	}
	// Une valeur inconnue ne doit pas rendre une liste vide ni désordonnée.
	if got := ordre(trierParticipation(lignes, "n_importe_quoi")); got != "2 3 4 1" {
		t.Errorf("tri inconnu : %q, attendu le tri par nom", got)
	}
	// L'entrée n'est pas remaniée en place : l'appelant peut la réutiliser.
	if ordre(lignes) != "1 2 3 4" {
		t.Errorf("la liste d'origine a été triée : %q", ordre(lignes))
	}
}

// L'espace d'administration se signale dans le fil : la rubrique seule ne
// disait pas qu'on avait quitté les commandes.
func TestRubriqueAdministration(t *testing.T) {
	dedans := []string{"admin", "member", "distribution", "contract", "amapadmin"}
	for _, c := range dedans {
		if !rubriqueAdministration(c) {
			t.Errorf("« %s » devrait relever de l'administration", c)
		}
	}
	for _, c := range []string{"home", "shop", "account", "", "public"} {
		if rubriqueAdministration(c) {
			t.Errorf("« %s » ne devrait pas relever de l'administration", c)
		}
	}
}

// Le cran « Espace d'administration » précède la rubrique, et n'apparaît pas
// sur les écrans qui n'en font pas partie.
func TestFilPorteLEspaceAdministration(t *testing.T) {
	base := func(cat string) PageData {
		return PageData{
			Group:      &model.Group{ID: 1, Name: "AMAP"},
			User:       &model.User{ID: 9, FirstName: "Alix", LastName: "Viginier"},
			Category:   cat,
			Breadcrumb: []BreadcrumbItem{{Name: "Membres", Link: "/member"}},
		}
	}
	dedans := renderNav(t, base("member"))
	if !strings.Contains(dedans, `>Espace d'administration</a>`) {
		t.Error("le fil d'un écran d'administration devrait porter ce cran")
	}
	if strings.Index(dedans, "Espace d'administration") > strings.Index(dedans, ">Membres<") {
		t.Error("le cran devrait précéder la rubrique")
	}

	dehors := base("home")
	dehors.Breadcrumb = []BreadcrumbItem{{Name: "Commandes", Link: "/home"}}
	if strings.Contains(renderNav(t, dehors), "Espace d'administration</a>") {
		t.Error("l'accueil ne fait pas partie de l'espace d'administration")
	}
}

// Les paramètres du groupe se consultent avec le droit « paramètres », mais
// ne se modifient que par le responsable : le formulaire est désactivé pour
// les autres, et son bouton d'enregistrement absent.
func TestParametresDuGroupeReservesAuResponsable(t *testing.T) {
	rendu := func(responsable bool) string {
		pd := PageData{
			Group:          &model.Group{ID: 1, Name: "AMAP"},
			User:           &model.User{ID: 9, FirstName: "Alix", LastName: "Viginier"},
			Category:       "amapadmin",
			HasParameters:  true,
			IsGroupManager: responsable,
		}
		tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html",
			"cycles_style.html", "amapadmin_layout.html", "amapadmin.html")
		if err != nil {
			t.Fatalf("parse : %v", err)
		}
		var sb strings.Builder
		if err := tpl.ExecuteTemplate(&sb, "base", pd); err != nil {
			t.Fatalf("render : %v", err)
		}
		return sb.String()
	}

	chef := rendu(true)
	if strings.Contains(chef, "<fieldset disabled>") {
		t.Error("le responsable doit pouvoir saisir")
	}
	if !strings.Contains(chef, "Enregistrer les paramètres") {
		t.Error("le responsable doit avoir le bouton d'enregistrement")
	}

	autre := rendu(false)
	if !strings.Contains(autre, "<fieldset disabled>") {
		t.Error("le formulaire devrait être désactivé sans le rôle de responsable")
	}
	if strings.Contains(autre, "Enregistrer les paramètres") {
		t.Error("le bouton d'enregistrement ne devrait pas s'afficher")
	}
	// Fragment court : la phrase est répartie sur deux lignes du gabarit.
	if !strings.Contains(autre, "peut en changer") {
		t.Error("la page devrait dire pourquoi la saisie est fermée")
	}
}

// Le menu latéral ne reprend pas les domaines qui vivent à l'intérieur d'un
// autre, et la vue d'ensemble signale ce qui se manipule sans garde-fou.
func TestTuilesHorsMenuEtDanger(t *testing.T) {
	tiles := adminTilesFor(PageData{IsGroupManager: true, HasParameters: true,
		CanManageRights: true, HasDatabaseAdmin: true})
	var droits, bdd *AdminTile
	for i := range tiles {
		switch tiles[i].Title {
		case "Droits":
			droits = &tiles[i]
		case "Base de données":
			bdd = &tiles[i]
		}
	}
	if droits == nil || bdd == nil {
		t.Fatal("les tuiles « Droits » et « Base de données » devraient exister")
	}
	if !droits.HorsMenu {
		t.Error("« Droits » est un onglet des paramètres : pas d'entrée de premier rang")
	}
	if !bdd.HorsMenu {
		t.Error("« Base de données » est un onglet des paramètres : pas d'entrée de premier rang")
	}
	if !bdd.Danger || bdd.Avertissement == "" {
		t.Error("« Base de données » doit se signaler et dire ce qu'on risque")
	}

	// Toute icône employée doit exister dans la fonte : une classe absente ne
	// rend rien, et rien ne le signale.
	css, err := os.ReadFile("../../www/font/icons.css")
	if err != nil {
		t.Skipf("fonte introuvable : %v", err)
	}
	for _, tl := range tiles {
		if !strings.Contains(string(css), "."+tl.Icon+":before") {
			t.Errorf("« %s » : l'icône %s n'existe pas dans la fonte", tl.Title, tl.Icon)
		}
	}
}
