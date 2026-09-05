package handler

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"
)

// Quitter un groupe efface une inscription. Servi en GET, il suffisait de
// charger l'adresse — un lien envoyé, une image, le préchargeur du navigateur —
// pour être retiré du groupe sans l'avoir demandé. Le départ passe désormais
// par un POST, et le gabarit ne propose plus le lien qui l'armait.
func TestQuitterLeGroupePasseParUnPost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, nil, &config.Config{})

	var enPost bool
	for _, route := range r.Routes() {
		if route.Path == "/account/quit" && route.Method == "POST" {
			enPost = true
		}
	}
	if !enPost {
		t.Error("POST /account/quit n'est pas enregistrée : le départ resterait en GET")
	}

	tpl, err := loadTemplatesFromRoot(t, "cycles_style.html", "account_quit.html")
	if err != nil {
		t.Fatalf("gabarit : %v", err)
	}
	var out strings.Builder
	pd := PageData{
		Group: &model.Group{ID: 1, Name: "AMAP"},
		User:  &model.User{ID: 9, FirstName: "Marie", LastName: "Dupont"},
	}
	if err := tpl.ExecuteTemplate(&out, "content", pd); err != nil {
		t.Fatalf("rendu : %v", err)
	}
	page := out.String()
	if strings.Contains(page, "token=") {
		t.Error("le gabarit propose encore un lien de confirmation en GET")
	}
	if !strings.Contains(page, `<form method="POST" action="/account/quit"`) {
		t.Error("le départ ne part plus d'un formulaire POST")
	}
}

// Le menu du compte nomme le groupe où l'on se trouve : « changer de groupe »
// tout seul ne disait pas lequel on quitterait.
func TestLeMenuDuCompteNommeLeGroupe(t *testing.T) {
	out := renderNav(t, PageData{
		Group: &model.Group{ID: 1, Name: "Alterconso du Val de Brenne"},
		User:  &model.User{ID: 9, FirstName: "Marie", LastName: "Dupont"},
	})

	if !strings.Contains(out, "Alterconso du Val de Brenne") {
		t.Error("le groupe courant n'est pas nommé dans le menu du compte")
	}
	if !strings.Contains(out, `href="/user/choose"`) {
		t.Error("on ne peut plus changer de groupe depuis le menu")
	}
	// La déconnexion se distingue des entrées de navigation qui la précèdent.
	if !strings.Contains(out, `href="/user/logout" class="sortie"`) {
		t.Error("la déconnexion a perdu sa mise à l'écart")
	}
}

// Écrire à un adhérent nommé est un geste de gestion : le bouton ne s'offre
// qu'à qui détient la gestion des membres. Le serveur refuse d'armer le
// destinataire aux autres — ce test garde la moitié visible de la règle, pour
// qu'on ne propose pas un bouton qui mènerait à un refus.
func TestLeBoutonEcrireSuitLaGestionDesMembres(t *testing.T) {
	rendre := func(pd PageData) string {
		t.Helper()
		tpl, err := loadTemplatesFromRoot(t, "cycles_style.html", "member.html")
		if err != nil {
			t.Fatalf("gabarit : %v", err)
		}
		var out strings.Builder
		if err := tpl.ExecuteTemplate(&out, "lignesMembres", pd); err != nil {
			t.Fatalf("rendu : %v", err)
		}
		return out.String()
	}

	base := func() PageData {
		return PageData{
			Group:   &model.Group{ID: 1, Name: "AMAP"},
			User:    &model.User{ID: 9, FirstName: "Alix", LastName: "Viginier"},
			Members: []MemberView{{ID: 7, FirstName: "Iris", LastName: "Adi", Email: "iris@example.org"}},
		}
	}

	if strings.Contains(rendre(base()), "/messages?member=7") {
		t.Error("le bouton d'écriture s'affiche sans la gestion des membres")
	}

	pd := base()
	pd.HasMembership = true
	avec := rendre(pd)
	if !strings.Contains(avec, "/messages?member=7") {
		t.Error("le gestionnaire des membres ne peut plus écrire à un adhérent")
	}
	// L'ordre des actions : se connecter, écrire, modifier, retirer.
	pd.IsGroupManager = true
	ordre := []string{"/member/loginAs/7", "/messages?member=7", "/member/edit/7", "/member/delete/7"}
	page, pos := rendre(pd), 0
	for _, lien := range ordre {
		i := strings.Index(page[pos:], lien)
		if i < 0 {
			t.Fatalf("%s absent de la ligne, ou hors de l'ordre attendu", lien)
		}
		pos += i
	}
}

// Les numéros arrivent tels qu'ils ont été tapés. On les redit par paires,
// sauf ceux qu'on ne reconnaît pas : un numéro étranger redécoupé de travers
// serait pire que le même, laissé tel quel.
func TestMiseEnFormeDesTelephones(t *testing.T) {
	cas := map[string]string{
		"0385723092":       "03 85 72 30 92",
		"06 84 56 27  90":  "06 84 56 27 90",
		"07.85.31.57.82":   "07 85 31 57 82",
		"+33683153040":     "06 83 15 30 40",
		"00 41 792377328":  "00 41 792377328", // suisse : intouché
		"0":                "0",               // saisie fautive : visible comme telle
		"":                 "",
		"  06 60 90 66 29": "06 60 90 66 29",
	}
	for entree, attendu := range cas {
		if got := formatTelephone(entree); got != attendu {
			t.Errorf("formatTelephone(%q) = %q, attendu %q", entree, got, attendu)
		}
	}
}

// La case « Recaler tous les producteurs sur ces horaires » était lue en
// comparant sa valeur à « on », alors que le gabarit l'écrit `value="1"` : la
// propagation ne se faisait jamais, et le gestionnaire qui repoussait
// l'ouverture des commandes ne voyait rien changer sur l'accueil. Le test
// tient les deux bouts — ce que le gabarit envoie, ce que le handler accepte.
func TestLaCaseDeRecalageEstLuePourCeQueLeGabaritEnvoie(t *testing.T) {
	if caseCochee("") {
		t.Error("une case décochée n'envoie rien : elle ne doit pas compter pour cochée")
	}
	for _, v := range []string{"on", "1", "true"} {
		if !caseCochee(v) {
			t.Errorf("une case cochée envoyant %q doit compter pour cochée", v)
		}
	}

	// Le répertoire courant dépend des tests déjà passés : on se replace à la
	// racine, comme le font ceux qui chargent des gabarits.
	chdirRepoRoot(t)
	source, err := os.ReadFile("templates/distribution_edit_md.html")
	if err != nil {
		t.Fatalf("lecture du gabarit : %v", err)
	}
	m := regexp.MustCompile(`name="syncAll"\s+value="([^"]*)"`).FindStringSubmatch(string(source))
	if m == nil {
		t.Fatal("la case syncAll a disparu du gabarit, ou n'a plus de valeur explicite")
	}
	if !caseCochee(m[1]) {
		t.Errorf("le gabarit envoie %q, que le handler ne reconnaît pas comme coché", m[1])
	}
}
