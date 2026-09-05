package service

import (
	"strings"
	"testing"
	"time"

	"github.com/gpenaud/alterconso/internal/model"
)

func distribFixture() model.MultiDistrib {
	end := time.Date(2026, 9, 10, 20, 0, 0, 0, time.UTC)
	return model.MultiDistrib{
		Group:            model.Group{Name: "AMAP du Pré"},
		Place:            model.Place{Name: "Salle des fêtes"},
		DistribStartDate: time.Date(2026, 9, 12, 18, 0, 0, 0, time.UTC),
		OrderEndDate:     &end,
	}
}

// Ce que le responsable écrit est du texte. Le rendre tel quel laisserait
// glisser un lien ou du balisage dans un courrier que le groupe signe.
func TestWrittenTextNeverBecomesMarkup(t *testing.T) {
	msg := CycleMessage{
		Subject:     "Ouverture",
		Body:        `Bonjour <script>alert(1)</script> et <a href="http://ailleurs.example">ici</a>`,
		ButtonLabel: "Commander",
	}
	out, err := renderCycleEmail(distribFixture(), model.User{FirstName: "Marie"}, msg, "alterconso.test")
	if err != nil {
		t.Fatalf("rendu : %v", err)
	}
	if strings.Contains(out, "<script>") {
		t.Error("le balisage saisi est passé tel quel")
	}
	if strings.Contains(out, `href="http://ailleurs.example"`) {
		t.Error("un lien saisi est devenu cliquable")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("le texte devrait apparaître échappé")
	}
}

// Une accolade tapée par mégarde ne doit pas faire échouer l'envoi, et une
// expression volontaire ne doit pas ouvrir les données du modèle.
func TestWrittenTextIsNotATemplate(t *testing.T) {
	msg := CycleMessage{Subject: "o", Body: "Rendez-vous {{.ShopURL}} {{ à 18h", ButtonLabel: "b"}
	out, err := renderCycleEmail(distribFixture(), model.User{FirstName: "Paul"}, msg, "alterconso.test")
	if err != nil {
		t.Fatalf("une accolade ne doit pas casser le rendu : %v", err)
	}
	if !strings.Contains(out, "{{.ShopURL}}") {
		t.Error("l'expression devrait s'afficher littéralement")
	}
}

// Les retours à la ligne survivent : un texte écrit en paragraphes qui arrive
// en un seul bloc ne se lit pas.
func TestLineBreaksSurvive(t *testing.T) {
	msg := CycleMessage{Subject: "o", Body: "Première ligne\r\nSeconde ligne", ButtonLabel: "b"}
	out, _ := renderCycleEmail(distribFixture(), model.User{FirstName: "A"}, msg, "alterconso.test")
	if !strings.Contains(out, "Première ligne<br>Seconde ligne") {
		t.Error("les retours à la ligne n'ont pas été rendus")
	}
}

// Le bouton mène à la boutique de LA distribution annoncée, et son libellé est
// celui qu'on a choisi. L'adresse, elle, ne se configure pas.
//
// Il passe par le sas et non par /shop/:id : cette dernière est servie par la
// SPA, qui répond 200 sans session et n'échoue qu'ensuite sur ses appels d'API.
// Un courrier se lit souvent après l'expiration du cookie.
func TestButtonPointsToTheDistributionShop(t *testing.T) {
	md := distribFixture()
	md.ID = 951
	msg := CycleMessage{Subject: "o", Body: "texte", ButtonLabel: "Je commande"}
	out, _ := renderCycleEmail(md, model.User{FirstName: "A"}, msg, "alterconso.test")

	if !strings.Contains(out, `href="https://alterconso.test/distribution/order/951"`) {
		t.Error("le bouton devrait mener à la boutique de cette distribution, via le sas")
	}
	if strings.Contains(out, `href="https://alterconso.test/shop/951"`) {
		t.Error("le lien direct laisserait une session expirée devant une page vide")
	}
	if !strings.Contains(out, "Je commande") {
		t.Error("le libellé choisi devrait figurer sur le bouton")
	}
	// Le réglage des notifications reste sur le compte, pas sur la boutique.
	if !strings.Contains(out, `href="https://alterconso.test/account"`) {
		t.Error("le lien vers l'espace personnel a disparu")
	}
}

// L'image est désignée par une adresse absolue : un chemin relatif ne mène
// nulle part depuis une boîte mail, qui ignore de quel domaine vient le message.
func TestImageURLIsAbsolute(t *testing.T) {
	msg := CycleMessage{Subject: "o", Body: "t", ButtonLabel: "b",
		ImageURL: "https://alterconso.test/file/12_abc.png"}
	out, _ := renderCycleEmail(distribFixture(), model.User{FirstName: "A"}, msg, "alterconso.test")
	if !strings.Contains(out, `src="https://alterconso.test/file/12_abc.png"`) {
		t.Error("l'image devrait être désignée par son adresse absolue")
	}

	// Sans image, aucune balise vide ne doit rester.
	sans, _ := renderCycleEmail(distribFixture(), model.User{FirstName: "A"},
		CycleMessage{Subject: "o", Body: "t", ButtonLabel: "b"}, "alterconso.test")
	if strings.Contains(sans, "<img") {
		t.Error("aucune balise image ne devrait subsister sans image")
	}
}

// Le libellé par défaut : un bouton sans texte ne se cliquerait pas.
func TestButtonLabelFallsBack(t *testing.T) {
	m := model.CycleMessage{}
	if m.ButtonLabel() != "Passer ma commande" {
		t.Errorf("libellé par défaut attendu, obtenu %q", m.ButtonLabel())
	}
	m.LinkLabel = "Voir le catalogue"
	if m.ButtonLabel() != "Voir le catalogue" {
		t.Error("le libellé choisi devrait primer")
	}
}

// Un courrier incomplet ou désactivé ne part pas : le gabarit d'origine prend
// alors le relais, plutôt qu'une page blanche.
func TestOnlyCompleteAndEnabledMessagesAreSent(t *testing.T) {
	cases := []struct {
		nom  string
		msg  model.CycleMessage
		want bool
	}{
		{"complet et actif", model.CycleMessage{Enabled: true, Subject: "o", Body: "b"}, true},
		{"désactivé", model.CycleMessage{Enabled: false, Subject: "o", Body: "b"}, false},
		{"sans objet", model.CycleMessage{Enabled: true, Body: "b"}, false},
		{"sans texte", model.CycleMessage{Enabled: true, Subject: "o"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			if got := tc.msg.IsSendable(); got != tc.want {
				t.Errorf("obtenu %v, attendu %v", got, tc.want)
			}
		})
	}
}
