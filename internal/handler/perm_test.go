package handler

import (
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

func ug(rightsJSON string) *model.UserGroup {
	return &model.UserGroup{Rights: rightsJSON}
}

func TestAuthorize(t *testing.T) {
	manager := ug(`[{"right":"GroupAdmin"}]`)
	messages := ug(`[{"right":"Messages"}]`)
	catalog := ug(`[{"right":"CatalogAdmin"}]`)
	none := ug(`[]`)

	cases := []struct {
		name   string
		ug     *model.UserGroup
		rights []model.Right
		want   bool
	}{
		{"nil ⇒ refus (fail-closed)", nil, []model.Right{model.RightMessages}, false},
		{"gestionnaire ⇒ accès quel que soit le droit", manager, []model.Right{model.RightMessages}, true},
		{"gestionnaire ⇒ accès même en manager-only", manager, nil, true},
		{"droit Messages requis, possédé", messages, []model.Right{model.RightMessages}, true},
		{"droit Messages requis, non possédé (a CatalogAdmin)", catalog, []model.Right{model.RightMessages}, false},
		{"manager-only, simple membre ⇒ refus", messages, nil, false},
		{"aucun droit du tout ⇒ refus", none, []model.Right{model.RightMessages}, false},
		{"un des droits parmi plusieurs suffit", catalog,
			[]model.Right{model.RightMessages, model.RightCatalogAdmin}, true},
	}
	for _, tc := range cases {
		if got := authorize(tc.ug, tc.rights); got != tc.want {
			t.Errorf("%s : authorize=%v, attendu %v", tc.name, got, tc.want)
		}
	}
}

// TestDistributionsAccedeAuxCommandes fige l'accès du droit « Gestion des
// distributions » aux commandes d'une date.
//
// Ces pages vivent sous « /contractAdmin » par héritage et étaient gardées par
// le seul droit « catalogues » : celui qui tient la distribution se voyait
// refuser la liste des commandes du jour, alors que la page de distribution lui
// en affiche le nombre et lui propose le lien. Seuls le responsable de groupe et
// le responsable technique passaient.
func TestDistributionsAccedeAuxCommandes(t *testing.T) {
	distributions := ug(`[{"right":"Distributions"}]`)
	catalog := ug(`[{"right":"CatalogAdmin"}]`)
	manager := ug(`[{"right":"GroupAdmin"}]`)
	membre := ug(`[{"right":"Messages"}]`)

	// Le garde des routes « ordersByDate », « vendorsByDate », leur export et
	// les actions qui corrigent une commande sur place.
	commandes := []model.Right{model.RightCatalogAdmin, model.RightDistributions}

	cases := []struct {
		name string
		ug   *model.UserGroup
		want bool
	}{
		{"gestion des distributions ⇒ accès", distributions, true},
		{"gestion des catalogues ⇒ accès (inchangé)", catalog, true},
		{"responsable de groupe ⇒ accès", manager, true},
		{"simple membre ⇒ refus", membre, false},
		{"non-membre ⇒ refus", nil, false},
	}
	for _, tc := range cases {
		if got := authorize(tc.ug, commandes); got != tc.want {
			t.Errorf("%s : authorize=%v, attendu %v", tc.name, got, tc.want)
		}
	}
}

// TestDistributionsNeDonnePasLeCatalogue borne la correction : ouvrir les
// commandes d'une date ne doit pas ouvrir l'édition des produits et des prix,
// qui reste au droit « catalogues ».
func TestDistributionsNeDonnePasLeCatalogue(t *testing.T) {
	distributions := ug(`[{"right":"Distributions"}]`)
	catalogueSeul := []model.Right{model.RightCatalogAdmin}

	if authorize(distributions, catalogueSeul) {
		t.Error("le droit distributions ne doit pas ouvrir l'administration des catalogues")
	}
	if !ug(`[{"right":"Distributions"}]`).CanManageDistributions() {
		t.Error("CanManageDistributions doit reconnaître le droit Distributions")
	}
	if ug(`[{"right":"Distributions"}]`).IsGroupManager() {
		t.Error("le droit distributions ne fait pas de son porteur un responsable de groupe")
	}
}
