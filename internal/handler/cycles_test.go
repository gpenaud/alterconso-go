package handler

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/model"
)

// Les écrans du cycle s'ouvrent à la délégation « distributions » : c'est le
// calendrier qu'on administre ici, et le courrier qui l'accompagne.
func TestCycleRoutesAreUnderTheDistributionsDelegation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, nil, &config.Config{})

	want := []string{
		"GET /distribution/cycles",
		"GET /distribution/cycles/:id/message",
		"POST /distribution/cycles/:id/message",
	}
	seen := map[string]bool{}
	for _, route := range r.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("%s n'est pas enregistrée", w)
		}
	}
}

// La périodicité se dit en mots : « 14 » ne se lit pas dans une liste.
func TestRhythmIsSpelledOut(t *testing.T) {
	for days, want := range map[int]string{
		7:  "Toutes les semaines",
		14: "Tous les quinze jours",
		21: "Toutes les trois semaines",
		30: "Tous les mois",
	} {
		if got := rhythmLabel(days); got != want {
			t.Errorf("%d jours : obtenu %q, attendu %q", days, got, want)
		}
	}
	// Une valeur inattendue se dit quand même, plutôt que de ne rien afficher.
	if got := rhythmLabel(5); !strings.Contains(got, "5") {
		t.Errorf("une périodicité inhabituelle doit rester lisible, obtenu %q", got)
	}
}

// L'écran annonce ce qu'il fait : le courrier remplace le message par défaut,
// et le lien n'est pas un champ libre.
func TestMessageScreenStatesWhatItDoes(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "distribution_cycle_message.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	data := CycleMessageData{
		PageData: PageData{
			User: &model.User{ID: 1, FirstName: "Alice"}, Group: &model.Group{ID: 1, Name: "AMAP"},
			HasDistributions: true,
		},
		Cycle:      model.DistributionCycle{ID: 3, Name: "Jeudi", Place: model.Place{Name: "Salle"}},
		Message:    model.CycleMessage{Subject: "Ouverture", Body: "Bonjour"},
		Categories: []string{"Membres réguliers", "Membres inactifs"},
		RhythmText: "Toutes les semaines",
	}

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", data); err != nil {
		t.Fatalf("render : %v", err)
	}
	out := sb.String()

	for _, attendu := range []string{
		`name="subject"`, `name="body"`, `name="image"`, `name="linkLabel"`,
		`name="recipientCategory"`, `name="enabled"`,
		`enctype="multipart/form-data"`,
		"Membres inactifs",
		"remplace alors le message d",
	} {
		if !strings.Contains(out, attendu) {
			t.Errorf("%s absent de l'écran", attendu)
		}
	}

	// L'adresse du bouton ne se saisit pas : un lien libre dans un courrier que
	// le groupe signe est exactement ce qui fabrique un hameçon crédible.
	if strings.Contains(out, `name="linkUrl"`) || strings.Contains(out, `name="url"`) {
		t.Error("l'adresse du lien ne doit pas être un champ")
	}
}

// La suppression détruit des journées de calendrier : elle passe par un
// formulaire POST. En lien GET, le préchargement d'un navigateur suffirait.
func TestCycleDeletionIsNotAClickableLink(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, nil, &config.Config{})

	var post, get bool
	for _, route := range r.Routes() {
		if route.Path == "/distribution/cycles/:id/delete" {
			post = post || route.Method == "POST"
			get = get || route.Method == "GET"
		}
	}
	if !post {
		t.Error("la suppression devrait être enregistrée en POST")
	}
	if get {
		t.Error("aucune route GET ne doit supprimer un cycle")
	}
}

// La colonne « Courrier » ne dit que ce qui part. Un courrier rédigé mais dont
// l'envoi est décoché n'a rien à y annoncer : le signaler laissait croire qu'il
// attendait une action.
func TestOnlyActiveMailIsAnnounced(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "distribution_cycles.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	data := CyclesData{PageData: PageData{
		User: &model.User{ID: 1, FirstName: "Alice"}, Group: &model.Group{ID: 1, Name: "AMAP"},
		HasDistributions: true,
	}}
	data.Cycles = []CycleRow{
		{ID: 1, Name: "Actif", MessageState: "Actif", HasMessage: true},
		{ID: 2, Name: "Rédigé mais éteint", MessageState: "", HasMessage: true},
		{ID: 3, Name: "Sans courrier", MessageState: "", HasMessage: false},
	}

	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", data); err != nil {
		t.Fatalf("render : %v", err)
	}
	out := sb.String()

	if !strings.Contains(out, "Actif") {
		t.Error("un courrier qui part doit être annoncé")
	}
	for _, absent := range []string{"Brouillon", "Aucun courrier"} {
		if strings.Contains(out, absent) {
			t.Errorf("%q ne doit plus figurer dans la colonne", absent)
		}
	}
	// La suppression est offerte sur chaque ligne.
	if !strings.Contains(out, `action="/distribution/cycles/2/delete"`) {
		t.Error("chaque cycle devrait pouvoir être supprimé")
	}
}
