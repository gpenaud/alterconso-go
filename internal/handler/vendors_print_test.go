package handler

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

// La version imprimable des totaux par producteur suit les options de la
// liste d'émargement : tout à la suite, un producteur par page, sans prix.
func TestVendorsByDatePrintModes(t *testing.T) {
	chdirRepoRoot(t)
	tpl, err := loadTemplates("vendors_by_date_print.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}
	render := func(mode, fontSize string) string {
		data := VendorsByDateData{
			PageData:  PageData{Group: &model.Group{ID: 7, Name: "AMAP"}, User: &model.User{ID: 1}},
			GroupName: "AMAP",
			DayLabel:  "jeudi 9 octobre 2026",
			DateISO:   "2026-10-09",
			Mode:      mode,
			FontSize:  fontSize,
			Vendors: []VendorByDateEntry{
				{CatalogName: "Légumes", VendorName: "Ferme du Jointout", Total: 12.5,
					Lines: []VendorByDateLine{{Qty: "3 × 500 g", Ref: "CAR", Product: "Carottes", UnitPrice: 2.5, Total: 7.5}}},
				{CatalogName: "Pain", VendorName: "Le Fournil", Total: 5,
					Lines: []VendorByDateLine{{Qty: "2", Product: "Pain complet", UnitPrice: 2.5, Total: 5}}},
			},
			GrandTotal: 17.5,
		}
		var b bytes.Buffer
		if err := tpl.ExecuteTemplate(&b, "vendors_by_date_print.html", data); err != nil {
			t.Fatalf("render %s : %v", mode, err)
		}
		return b.String()
	}

	all := render("all", "M")
	for _, want := range []string{"Ferme du Jointout", "Le Fournil", "Carottes", "7.50 €", "Total de toutes les commandes : 17.50 €",
		`onclick="window.print()"`, "/contractAdmin/vendorsByDate/2026-10-09/7/printOptions", "font-size: 12px"} {
		if !strings.Contains(all, want) {
			t.Errorf("mode all : %q manque", want)
		}
	}
	if strings.Contains(all, `class="perpage"`) {
		t.Error("mode all : pas de saut de page")
	}

	perpage := render("perpage", "XL")
	if !strings.Contains(perpage, `class="perpage"`) || !strings.Contains(perpage, "font-size: 17px") {
		t.Error("mode perpage : la classe de saut de page ou la police XL manque")
	}

	noprice := render("noprice", "S")
	for _, absent := range []string{"7.50 €", "P.U.", "Total de toutes les commandes"} {
		if strings.Contains(noprice, absent) {
			t.Errorf("mode noprice : %q ne devrait pas apparaître", absent)
		}
	}
	if !strings.Contains(noprice, "Carottes") || !strings.Contains(noprice, "font-size: 10px") {
		t.Error("mode noprice : les produits et la police S doivent rester")
	}
}

// L'écran d'options mène à la page imprimable avec les valeurs par défaut
// de l'émargement : tout à la suite, police M.
func TestVendorsByDatePrintOptionsForm(t *testing.T) {
	chdirRepoRoot(t)
	tpl, err := loadTemplates("base.html", "design.html", "vendors_by_date_config.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}
	var b bytes.Buffer
	data := VendorsByDateData{
		PageData: PageData{Group: &model.Group{ID: 7, Name: "AMAP"}, User: &model.User{ID: 1}},
		DateISO:  "2026-10-09", DayLabel: "jeudi 9 octobre 2026",
	}
	if err := tpl.ExecuteTemplate(&b, "base", data); err != nil {
		t.Fatalf("render : %v", err)
	}
	out := b.String()
	for _, want := range []string{`action="/contractAdmin/vendorsByDate/2026-10-09/7/print"`,
		`name="mode" value="all" checked`, `name="fontSize" value="M" checked`, `name="mode" value="perpage"`, `name="mode" value="noprice"`,
		// On arrive ici depuis la liste des commandes : Retour y ramène, et la
		// vue écran des totaux reste à un clic.
		`href="/contractAdmin/ordersByDate/2026-10-09/7"`, `href="/contractAdmin/vendorsByDate/2026-10-09/7"`} {
		if !strings.Contains(out, want) {
			t.Errorf("%q manque", want)
		}
	}
}
