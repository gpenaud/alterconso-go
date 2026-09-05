package handler

import (
	"strings"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
)

// Les flèches de période se déplacent depuis la période affichée. Elles
// portaient -1 et 1 en dur : depuis l'origine le premier clic fonctionnait,
// puis chaque pression rejouait le même lien et la page ne bougeait plus.
func TestPeriodArrowsFollowCurrentOffset(t *testing.T) {
	chdirRepoRoot(t)

	cases := []struct {
		nom      string
		page     string
		tpl      string
		offset   int
		wantPrev string
		wantNext string
	}{
		{"distributions, période courante", "/distribution", "distribution.html", 0, "offset=-1", "offset=1"},
		{"distributions, trois crans plus loin", "/distribution", "distribution.html", 3, "offset=2", "offset=4"},
		{"distributions, dans le passé", "/distribution", "distribution.html", -2, "offset=-3", "offset=-1"},
		{"accueil, deux crans plus loin", "/home", "home.html", 2, "offset=1", "offset=3"},
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			tpl, err := loadTemplates("base.html", "design.html", "cycles_style.html", tc.tpl)
			if err != nil {
				t.Fatalf("parse : %v", err)
			}
			pd := PageData{
				Group: &model.Group{ID: 1, Name: "AMAP"}, User: &model.User{ID: 9},
				IsGroupManager: true, PeriodLabel: "Du 1 au 30",
				PrevOffset: tc.offset - 1, NextOffset: tc.offset + 1,
			}
			var sb strings.Builder
			if err := tpl.ExecuteTemplate(&sb, "base", pd); err != nil {
				t.Fatalf("render : %v", err)
			}
			out := sb.String()
			for _, want := range []string{
				tc.page + "?" + tc.wantPrev + `"`,
				tc.page + "?" + tc.wantNext + `"`,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("%q absent du rendu", want)
				}
			}
		})
	}
}
