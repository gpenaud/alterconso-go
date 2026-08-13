package handler

import (
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

func TestSuggestPhone(t *testing.T) {
	phone := func(s string) *string { return &s }

	cases := []struct {
		nom  string
		user *model.User
		path string
		want bool
	}{
		{"numéro absent", &model.User{}, "/", true},
		{"numéro nul", &model.User{Phone: nil}, "/home", true},
		{"numéro vide", &model.User{Phone: phone("")}, "/home", true},
		{"numéro fait d'espaces", &model.User{Phone: phone("   ")}, "/home", true},
		{"numéro renseigné", &model.User{Phone: phone("06 12 34 56 78")}, "/home", false},
		// Le rappel se tait là où le champ est déjà sous les yeux.
		{"page d'édition du compte", &model.User{}, "/account/edit", false},
		{"visiteur non connecté", nil, "/", false},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := suggestPhone(tc.user, tc.path); got != tc.want {
				t.Errorf("suggestPhone = %v, attendu %v", got, tc.want)
			}
		})
	}
}
