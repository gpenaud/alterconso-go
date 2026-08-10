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
