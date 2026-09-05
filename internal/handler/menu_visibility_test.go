package handler

import (
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

// Le menu ne propose que ce que l'utilisateur peut faire. « Producteurs »
// n'offre aucune action a un adherent : l'entree encombrait son menu sans lui
// servir, et les entrees d'administration ne le concernent pas davantage.
func TestMenuHidesAdminEntriesFromPlainMember(t *testing.T) {
	out := renderNav(t, PageData{
		Group: &model.Group{ID: 1, Name: "AMAP"},
		User:  &model.User{ID: 9},
	})

	for _, absent := range []string{`href="/amap"`, `href="/distribution"`, `href="/amapadmin"`} {
		if strings.Contains(out, absent) {
			t.Errorf("%s ne devrait pas figurer dans le menu d'un adherent", absent)
		}
	}
	// Ce qui reste ouvert a tous ne doit pas disparaitre pour autant.
	for _, present := range []string{`href="/home"`, `href="/account"`, `href="/messages"`} {
		if !strings.Contains(out, present) {
			t.Errorf("%s a disparu du menu", present)
		}
	}
}

// L'onglet des producteurs se rouvre a tout adherent par configuration : un
// groupe qui s'en sert comme annuaire ne doit pas avoir a reprendre le code.
func TestVendorsTabReopensByConfiguration(t *testing.T) {
	membre := PageData{Group: &model.Group{ID: 1, Name: "AMAP"}, User: &model.User{ID: 9}}
	if strings.Contains(renderNav(t, membre), `href="/amap"`) {
		t.Error("ferme par defaut pour un adherent")
	}

	membre.ShowVendorsTab = true
	if !strings.Contains(renderNav(t, membre), `href="/amap"`) {
		t.Error("l'onglet devrait reparaitre une fois rouvert")
	}
}

// Chaque delegation ouvre son entree, et elle seule : c'est ce decoupage qui a
// remplace le pouvoir general d'autrefois.
func TestMenuFollowsDelegations(t *testing.T) {
	distributions := renderNav(t, PageData{
		Group: &model.Group{ID: 1, Name: "AMAP"}, User: &model.User{ID: 9},
		HasDistributions: true,
	})
	if !strings.Contains(distributions, `href="/distribution"`) {
		t.Error("la delegation « distributions » doit ouvrir son entree")
	}
	if strings.Contains(distributions, `href="/amapadmin"`) {
		t.Error("elle ne doit pas ouvrir les parametres")
	}
	// Une fonction dans le groupe rend l'ecran des producteurs utile.
	if !strings.Contains(renderNav(t, PageData{
		Group: &model.Group{ID: 1, Name: "AMAP"}, User: &model.User{ID: 9},
		HasDistributions: true, ShowVendorsTab: true,
	}), `href="/amap"`) {
		t.Error("les producteurs devraient etre visibles avec une delegation")
	}

	parametres := renderNav(t, PageData{
		Group: &model.Group{ID: 1, Name: "AMAP"}, User: &model.User{ID: 9},
		HasParameters: true,
	})
	if !strings.Contains(parametres, `href="/amapadmin"`) {
		t.Error("la delegation « parametres » doit ouvrir son entree")
	}
	if strings.Contains(parametres, `href="/distribution"`) {
		t.Error("elle ne doit pas ouvrir les distributions")
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
	tpl, err := loadTemplates("base.html", "design.html", "cycles_style.html", "home.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}
	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", pd); err != nil {
		t.Fatalf("render : %v", err)
	}
	return sb.String()
}
