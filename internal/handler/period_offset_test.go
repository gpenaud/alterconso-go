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
	}

	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			// cycles_style.html porte le gabarit de style que les écrans
			// refondus appellent : sans lui, le rendu s'arrête au premier
			// {{template "cyclesStyle"}}.
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

// L'accueil n'a plus de flèche vers le futur : le défilement continu y verse
// les périodes suivantes à mesure qu'on descend. Reste le lien vers le passé,
// qui doit suivre la période affichée — c'est le défaut d'origine, un lien
// figé rejouant la même page à chaque pression.
func TestHomeKeepsOnlyThePastLink(t *testing.T) {
	chdirRepoRoot(t)
	tpl, err := loadTemplates("base.html", "design.html", "cycles_style.html", "home.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	pd := PageData{
		Group: &model.Group{ID: 1, Name: "AMAP"}, User: &model.User{ID: 9},
		Category: "home", PeriodLabel: "Du 1 au 30",
		PrevOffset: 1, NextOffset: 3,
	}
	pd.MultiDistribs = []MultiDistribView{{ID: 1, Day: "12", DayLabelFull: "Jeudi 12",
		Place: "Salle", StartHour: "18:00", EndHour: "19:30"}}
	pd.HeroDistrib, pd.NextDistribs = splitDistribs(pd.MultiDistribs)

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", pd); err != nil {
		t.Fatalf("render : %v", err)
	}
	out := sb.String()

	if !strings.Contains(out, `/home?offset=1"`) {
		t.Error("le lien vers les distributions passées devrait suivre la période affichée")
	}
	if strings.Contains(out, `/home?offset=3"`) {
		t.Error("la flèche vers le futur fait double emploi avec le défilement continu")
	}
	// Et le déclencheur du défilement doit exister, même sans distribution
	// suivante dans la période courante.
	if !strings.Contains(out, `id="ac-sentinelle"`) {
		t.Error("le déclencheur du défilement a disparu")
	}
	if !strings.Contains(out, `id="ac-suivantes"`) {
		t.Error("la liste doit exister même vide, pour recevoir les périodes suivantes")
	}
}
