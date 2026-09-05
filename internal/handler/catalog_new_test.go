package handler

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"
)

// Un catalogue ne s'ouvrait que par duplication d'un autre : le premier
// catalogue d'un producteur nouvellement arrive n'avait aucun moyen d'exister.
func TestCatalogNewRouteMounts(t *testing.T) {
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
	for _, attendue := range []string{"GET /contractAdmin/new", "POST /contractAdmin/new"} {
		if !seen[attendue] {
			t.Errorf("%s n'est pas enregistrée", attendue)
		}
	}
}

func postCatalogue(valeurs url.Values) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/contractAdmin/new", strings.NewReader(valeurs.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req
	return c
}

func TestCatalogDepuisFormulaire(t *testing.T) {
	t.Run("les reglages sont repris", func(t *testing.T) {
		var cat model.Catalog
		catalogDepuisFormulaire(postCatalogue(url.Values{
			"name":                  {"  Légumes de saison  "},
			"vendor_id":             {"7"},
			"contact_id":            {"3"},
			"start_date":            {"2026-09-01"},
			"end_date":              {"2027-06-30"},
			"users_can_order":       {"1"},
			"stock_management":      {"1"},
			"percentage_fees":       {"1"},
			"percentage_fees_value": {"5"},
			"percentage_name":       {"Frais de fonctionnement"},
		}), &cat, true)

		if cat.Name != "Légumes de saison" {
			t.Errorf("nom non nettoye : %q", cat.Name)
		}
		if cat.VendorID != 7 {
			t.Errorf("producteur perdu : %d", cat.VendorID)
		}
		if cat.ContactID == nil || *cat.ContactID != 3 {
			t.Error("responsable perdu")
		}
		if cat.StartDate == nil || cat.StartDate.Format("2006-01-02") != "2026-09-01" {
			t.Error("date de debut perdue")
		}
		if !cat.HasFlag(model.CatalogFlagUsersCanOrder) || !cat.HasFlag(model.CatalogFlagStockManagement) {
			t.Error("options perdues")
		}
		if cat.PercentageFees == nil || *cat.PercentageFees != 5 {
			t.Error("pourcentage de frais perdu")
		}
	})

	// Les options decochees ne sont pas postees : elles doivent retomber,
	// et non rester allumees de la lecture precedente.
	t.Run("une option decochee s eteint", func(t *testing.T) {
		cat := model.Catalog{}
		cat.SetFlag(model.CatalogFlagStockManagement)
		cat.SetFlag(model.CatalogFlagHasPercentageFees)
		v := 12.0
		cat.PercentageFees = &v
		catalogDepuisFormulaire(postCatalogue(url.Values{"name": {"Pain"}}), &cat, true)
		if cat.HasFlag(model.CatalogFlagStockManagement) {
			t.Error("la gestion des stocks est restee allumee")
		}
		if cat.PercentageFees != nil {
			t.Errorf("les frais sont restes a %v", *cat.PercentageFees)
		}
	})

	// Sans le droit « distributions », le champ n'est pas meme lu : un
	// formulaire forge ne doit pas suffire la ou l'ecran n'affiche rien.
	t.Run("la mise en avant est ignoree sans le droit", func(t *testing.T) {
		var cat model.Catalog
		catalogDepuisFormulaire(postCatalogue(url.Values{
			"name":            {"Miel"},
			"highlight_label": {"Récolte de printemps"},
		}), &cat, false)
		if cat.HighlightLabel != nil {
			t.Errorf("mise en avant ecrite sans le droit : %q", *cat.HighlightLabel)
		}
	})

	t.Run("un libelle trop long est coupe a 48", func(t *testing.T) {
		var cat model.Catalog
		catalogDepuisFormulaire(postCatalogue(url.Values{
			"name":            {"Miel"},
			"highlight_label": {strings.Repeat("é", 60)},
		}), &cat, true)
		if cat.HighlightLabel == nil {
			t.Fatal("mise en avant perdue")
		}
		if n := len([]rune(*cat.HighlightLabel)); n != 48 {
			t.Errorf("libelle de %d caracteres, la colonne en tient 48", n)
		}
	})
}

// Les trois refus qui ne dependent pas de la base : ils coupent avant toute
// requete, et c'est voulu — rien ne sert d'interroger la base pour une saisie
// qu'on refuse de toute facon.
func TestCatalogNouveauRefus(t *testing.T) {
	h := &PagesHandler{}
	pd := PageData{Group: &model.Group{}}

	if h.catalogNouveauRefus(pd, model.Catalog{VendorID: 1}) == "" {
		t.Error("un catalogue sans nom a ete accepte")
	}
	if h.catalogNouveauRefus(pd, model.Catalog{Name: "Légumes"}) == "" {
		t.Error("un catalogue sans producteur a ete accepte")
	}

	debut := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	fin := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	inverse := model.Catalog{Name: "Légumes", VendorID: 1, StartDate: &debut, EndDate: &fin}
	if h.catalogNouveauRefus(pd, inverse) == "" {
		t.Error("un catalogue qui ferme avant d'ouvrir a ete accepte")
	}
}
