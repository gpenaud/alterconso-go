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
		"GET /distribution/cycles/new",
		"POST /distribution/cycles/new",
		"GET /distribution/cycles/:id",
		"POST /distribution/cycles/:id",
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

// Le rappel se règle depuis le formulaire du cycle, derrière une case à cocher
// qui le commande : c'est le même écran qui programme les journées et rédige
// le courrier qui les annonce.
func TestCycleFormCarriesTheReminderBehindACheckbox(t *testing.T) {
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "cycles_style.html", "distribution_cycle_form.html")
	if err != nil {
		t.Fatalf("parse : %v", err)
	}

	base := PageData{
		User: &model.User{ID: 1, FirstName: "Alice"}, Group: &model.Group{ID: 1, Name: "AMAP"},
		HasDistributions: true,
	}

	// Création : les champs de programmation, et le bloc replié par défaut.
	creation := CycleFormData{
		PageData: base, IsNew: true,
		Places:     []model.Place{{ID: 1, Name: "Salle"}},
		Categories: []string{"Membres réguliers"},
	}
	var sb strings.Builder
	if err := tpl.ExecuteTemplate(&sb, "base", creation); err != nil {
		t.Fatalf("render création : %v", err)
	}
	out := sb.String()

	for _, attendu := range []string{
		`name="cycleType"`, `name="startDate"`, `name="endDate"`, `name="placeId"`,
		`value="SemiAnnual"`, `value="Annual"`, `value="Monthly"`, `value="Weekly"`,
		`name="name"`, `id="enabledBox"`, `id="messageBlock"`,
		`name="subject"`, `name="body"`, `name="image"`, `name="recipientCategory"`,
		`action="/distribution/cycles/new"`,
	} {
		if !strings.Contains(out, attendu) {
			t.Errorf("%s absent du formulaire de création", attendu)
		}
	}
	// Sans rappel actif, le bloc est replié : la case le commande. Le dépliage
	// passe par la classe « ouvert », que le script pose et retire.
	if !strings.Contains(out, `class="cyc-repli " id="messageBlock"`) &&
		!strings.Contains(out, `class="cyc-repli" id="messageBlock"`) {
		t.Errorf("le bloc du rappel devrait être replié tant que la case n'est pas cochée")
	}

	// Modification : pas de replanification, mais une prolongation.
	edition := CycleFormData{
		PageData:   base,
		Cycle:      model.DistributionCycle{ID: 3, Name: "Jeudi", Place: model.Place{Name: "Salle"}},
		Message:    model.CycleMessage{Enabled: true, Subject: "Ouverture", Body: "Bonjour"},
		RhythmText: "Toutes les semaines",
	}
	sb.Reset()
	if err := tpl.ExecuteTemplate(&sb, "base", edition); err != nil {
		t.Fatalf("render modification : %v", err)
	}
	out = sb.String()

	if !strings.Contains(out, `name="extendTo"`) {
		t.Error("la modification devrait permettre de prolonger le cycle")
	}
	if strings.Contains(out, `name="cycleType"`) {
		t.Error("le rythme ne se replanifie pas : les journées portent des commandes")
	}
	if !strings.Contains(out, `action="/distribution/cycles/3"`) {
		t.Error("le formulaire de modification vise le cycle")
	}
	// Rappel actif : le bloc est déplié dès le rendu, sans attendre le script.
	if !strings.Contains(out, `cyc-repli ouvert`) {
		t.Error("le bloc devrait être déplié quand le rappel est actif")
	}

	// L'adresse du bouton ne se saisit pas, ici non plus.
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
	tpl, err := loadTemplatesFromRoot(t, "base.html", "design.html", "cycles_style.html", "distribution_cycles.html")
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
