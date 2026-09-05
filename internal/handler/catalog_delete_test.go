package handler

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"
)

func TestCatalogDeleteRouteIsPostOnly(t *testing.T) {
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
	if !seen["POST /contractAdmin/delete/:id"] {
		t.Error("la suppression d'un catalogue n'est pas enregistrée")
	}
	// La suppression emporte les produits et les dates de livraison : un lien
	// prélevé par un antivirus de messagerie suffirait à la déclencher.
	if seen["GET /contractAdmin/delete/:id"] {
		t.Error("la suppression d'un catalogue ne doit pas s'atteindre en GET")
	}
}

// Une commande passée retient le catalogue : ce sont des écritures comptables,
// et les effacer laisserait des soldes que plus rien n'explique. L'écran ne
// doit pas conduire à un refus — le bouton disparaît, et dit pourquoi.
func TestCatalogDeleteButtonFollowsOrders(t *testing.T) {
	chdirRepoRoot(t)

	base := PageData{
		User:     &model.User{ID: 1, FirstName: "Alix", LastName: "Viginier"},
		Group:    &model.Group{ID: 1, Name: "AMAP"},
		Category: "contract",
	}
	base.IsGroupManager = true
	base.AdminTiles = adminTilesFor(base)

	rendu := func(t *testing.T, commandes int64, gestionnaire bool) string {
		t.Helper()
		pd := base
		pd.IsGroupManager = gestionnaire
		data := CatalogEditData{
			CatalogAdminData: CatalogAdminData{
				PageData:  pd,
				Catalog:   model.Catalog{ID: 12, Name: "Légumes de saison"},
				ActiveTab: "view",
			},
			Commandes: commandes,
		}
		tpl, err := loadTemplates("base.html", "design.html", "cycles_style.html",
			"contractadmin_layout.html", "contractadmin_edit.html")
		if err != nil {
			t.Fatalf("parse : %v", err)
		}
		var sb strings.Builder
		if err := tpl.ExecuteTemplate(&sb, "base", data); err != nil {
			t.Fatalf("rendu interrompu : %v", err)
		}
		return sb.String()
	}

	t.Run("sans commande, le bouton est là", func(t *testing.T) {
		out := rendu(t, 0, true)
		if !strings.Contains(out, `action="/contractAdmin/delete/12"`) {
			t.Error("le catalogue sans commande ne peut pas être supprimé")
		}
	})

	t.Run("avec des commandes, le bouton disparaît et dit pourquoi", func(t *testing.T) {
		out := rendu(t, 3, true)
		if strings.Contains(out, `action="/contractAdmin/delete/12"`) {
			t.Error("un catalogue commandé s'offre à la suppression")
		}
		if !strings.Contains(out, "3 commandes ont") {
			t.Error("l'écran ne dit pas ce qui retient le catalogue")
		}
	})

	// Ouvrir un catalogue est réservé au responsable de groupe ; le fermer
	// pour de bon ne saurait être plus ouvert. Le responsable de catalogue,
	// qui tient les produits, atteint pourtant cet écran.
	t.Run("le responsable de catalogue ne voit pas la section", func(t *testing.T) {
		out := rendu(t, 0, false)
		if strings.Contains(out, "Supprimer ce catalogue") {
			t.Error("la suppression s'offre à qui ne répond pas du groupe")
		}
	})
}
