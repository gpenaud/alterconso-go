package handler

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"
)

// L'ecran des producteurs ne savait que lire. Ces routes sont ce qui lui rend
// l'ecriture ; elles voisinent /vendor/view/:id, et Gin refuse deux parametres
// differents au meme niveau en paniquant au demarrage.
func TestVendorWriteRoutesMount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("montage des routes impossible : %v", p)
		}
	}()
	Register(r, nil, &config.Config{})

	seen := map[string]bool{}
	for _, route := range r.Routes() {
		seen[route.Method+" "+route.Path] = true
	}

	for _, attendue := range []string{
		"GET /vendor/insert",
		"POST /vendor/insert",
		"GET /vendor/edit/:id",
		"POST /vendor/edit/:id",
		"POST /vendor/delete/:id",
	} {
		if !seen[attendue] {
			t.Errorf("%s n'est pas enregistrée", attendue)
		}
	}

	// La suppression efface une fiche que d'autres groupes lisent : un GET
	// la mettrait a portee d'un prechargement de lien.
	if seen["GET /vendor/delete/:id"] {
		t.Error("la suppression d'un producteur ne doit pas s'atteindre en GET")
	}
}

func formulaire(valeurs url.Values) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/vendor/insert", strings.NewReader(valeurs.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req
	return c
}

func TestVendorDepuisFormulaire(t *testing.T) {
	complet := func() url.Values {
		return url.Values{
			"name":        {"  Ferme des Trois Chenes  "},
			"email":       {" contact@trois-chenes.fr "},
			"phone":       {"0601020304"},
			"city":        {"Saint-Genest"},
			"legalStatus": {string(model.LegalStatusOrganization)},
			"organic":     {"1"},
		}
	}

	t.Run("les champs sont repris, espaces compris", func(t *testing.T) {
		var v model.Vendor
		if erreur := vendorDepuisFormulaire(formulaire(complet()), &v); erreur != "" {
			t.Fatalf("refus inattendu : %s", erreur)
		}
		if v.Name != "Ferme des Trois Chenes" {
			t.Errorf("nom non nettoye : %q", v.Name)
		}
		if v.Email != "contact@trois-chenes.fr" {
			t.Errorf("courriel non nettoye : %q", v.Email)
		}
		if v.City == nil || *v.City != "Saint-Genest" {
			t.Error("commune perdue")
		}
		if !v.Organic {
			t.Error("la case bio n'a pas ete lue")
		}
		if v.LegalStatus == nil || *v.LegalStatus != model.LegalStatusOrganization {
			t.Error("statut juridique perdu")
		}
	})

	// Un champ qu'on vide doit passer a NULL, et non rester a "" : la fiche
	// affiche « Non renseigne » sur un pointeur nil, pas sur une chaine vide.
	t.Run("un champ vide devient nil", func(t *testing.T) {
		vals := complet()
		vals.Set("phone", "   ")
		var v model.Vendor
		v.Phone = new(string)
		*v.Phone = "0100000000"
		if erreur := vendorDepuisFormulaire(formulaire(vals), &v); erreur != "" {
			t.Fatalf("refus inattendu : %s", erreur)
		}
		if v.Phone != nil {
			t.Errorf("le telephone efface reste a %q", *v.Phone)
		}
	})

	// Le formulaire ne propose que trois statuts ; une valeur forgee ne doit
	// pas entrer en base par la porte du POST.
	t.Run("un statut inconnu est ignore", func(t *testing.T) {
		vals := complet()
		vals.Set("legalStatus", "Cooperative")
		var v model.Vendor
		if erreur := vendorDepuisFormulaire(formulaire(vals), &v); erreur != "" {
			t.Fatalf("refus inattendu : %s", erreur)
		}
		if v.LegalStatus != nil {
			t.Errorf("statut forge accepte : %q", *v.LegalStatus)
		}
	})

	refus := map[string]struct {
		champ, valeur string
	}{
		"nom vide":              {"name", "   "},
		"courriel vide":         {"email", ""},
		"courriel sans arobase": {"email", "contact.trois-chenes.fr"},
		"courriel sans domaine": {"email", "contact@"},
		"courriel avec espace":  {"email", "contact @trois-chenes.fr"},
	}
	for nom, cas := range refus {
		t.Run(nom+" est refuse", func(t *testing.T) {
			vals := complet()
			vals.Set(cas.champ, cas.valeur)
			var v model.Vendor
			if erreur := vendorDepuisFormulaire(formulaire(vals), &v); erreur == "" {
				t.Error("la saisie a ete acceptee")
			}
		})
	}
}

// Le droit seul ne suffit pas : une fiche producteur ne porte pas de groupe, et
// « responsable de groupe » sans rattachement laisserait reecrire la ferme du
// groupe d'a cote. Sans droit, la reponse est non quoi qu'il arrive — verifie
// ici sans base, le premier test coupant avant toute requete.
func TestVendorEcrivableRefuseSansDroit(t *testing.T) {
	h := &PagesHandler{}
	if h.vendorEcrivable(PageData{Group: &model.Group{}}, 1) {
		t.Error("une fiche s'ouvre a qui n'a pas le droit de la modifier")
	}
	if h.vendorEcrivable(PageData{CanEditVendors: true}, 1) {
		t.Error("une fiche s'ouvre hors de tout groupe")
	}
}
